package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Frank5ive/goforge/benchmark/accept"
)

// Score is how well a service meets its task.
type Score struct {
	Built          bool            `json:"built"`
	Vetted         bool            `json:"vetted"`
	OwnTestsPassed bool            `json:"own_tests_passed"`
	Started        bool            `json:"started"`
	Passed         int             `json:"passed"`
	Total          int             `json:"total"`
	Checks         []accept.Result `json:"checks,omitempty"`
	GoLines        int             `json:"go_lines"`
	TestLines      int             `json:"test_lines"`
	// Notes explain failures to build, vet, test, or start.
	Notes []string `json:"notes,omitempty"`
}

// storage is where a task's service keeps its data.
type storage struct {
	url     string // DATABASE_URL for the service; "" for in-memory tasks
	testURL string // DATABASE_URL and TEST_DATABASE_URL for the service's own tests
	file    string // the SQLite database file, for SQLite tasks
}

// score builds the service in dir, runs go vet and its tests, starts it with
// the given storage, and runs the task's acceptance checks against it. It
// doesn't depend on Docker, so it runs the same inside the benchmark image as
// on a development machine.
func score(task, dir string, st storage) Score {
	var s Score
	suite := accept.Suites[task]
	s.Total = accept.Count(task)
	s.GoLines, s.TestLines = countLines(dir)

	for _, file := range []string{st.file, st.testURL} {
		if task == "products-sqlite" && file != "" {
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				s.Notes = append(s.Notes, err.Error())
				return s
			}
		}
	}

	work, err := os.MkdirTemp("", "bench-score-")
	if err != nil {
		s.Notes = append(s.Notes, err.Error())
		return s
	}
	defer os.RemoveAll(work)
	binary := filepath.Join(work, "service")

	if out, err := goCmd(dir, 5*time.Minute, nil, "build", "-o", binary, "."); err != nil {
		s.Notes = append(s.Notes, "go build failed: "+truncate(out))
		return s
	}
	s.Built = true

	if out, err := goCmd(dir, 5*time.Minute, nil, "vet", "./..."); err != nil {
		s.Notes = append(s.Notes, "go vet failed: "+truncate(out))
	} else {
		s.Vetted = true
	}

	// The service's own tests get their own storage, as in development
	var testEnv []string
	if st.testURL != "" {
		testEnv = []string{"DATABASE_URL=" + st.testURL, "TEST_DATABASE_URL=" + st.testURL}
	}
	if out, err := goCmd(dir, 10*time.Minute, testEnv, "test", "-count=1", "./..."); err != nil {
		s.Notes = append(s.Notes, "go test failed: "+truncate(out))
	} else {
		s.OwnTestsPassed = true
	}

	var env []string
	if st.url != "" {
		env = append(env, "DATABASE_URL="+st.url)
	}
	svc := &service{binary: binary, dir: dir, log: filepath.Join(work, "service.log"), env: env}
	if err := svc.start(); err != nil {
		s.Notes = append(s.Notes, "starting the service: "+err.Error())
		return s
	}
	defer svc.stop()
	s.Started = true

	s.Checks = suite(accept.Env{BaseURL: svc.url(), Restart: svc.restart, DatabaseFile: st.file})
	for _, c := range s.Checks {
		if c.Passed {
			s.Passed++
		}
	}
	return s
}

// service is a running build of the service under test.
type service struct {
	binary, dir, log string
	env              []string
	port             int
	cmd              *exec.Cmd
	exited           chan error // receives the result of cmd.Wait
}

func (s *service) url() string { return fmt.Sprintf("http://127.0.0.1:%d", s.port) }

// start runs the service on a free port and waits until GET /health responds 200.
func (s *service) start() error {
	if s.port == 0 {
		port, err := freePort()
		if err != nil {
			return err
		}
		s.port = port
	}
	logFile, err := os.OpenFile(s.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	s.cmd = exec.Command(s.binary)
	s.cmd.Dir = s.dir
	s.cmd.Env = append(os.Environ(), append(s.env, fmt.Sprintf("PORT=%d", s.port))...)
	s.cmd.Stdout, s.cmd.Stderr = logFile, logFile
	if err := s.cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	s.exited = make(chan error, 1)
	go func() { s.exited <- s.cmd.Wait(); logFile.Close() }()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-s.exited:
			s.cmd = nil
			return fmt.Errorf("the service exited (%v): %s", err, truncate(readFile(s.log)))
		default:
		}
		if resp, err := client.Get(s.url() + "/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.stop()
	return fmt.Errorf("GET /health did not respond 200 within 60s: %s", truncate(readFile(s.log)))
}

// stop sends SIGTERM and kills the service if it hasn't exited after 10s.
func (s *service) stop() {
	if s.cmd == nil {
		return
	}
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-s.exited:
	case <-time.After(10 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.exited
	}
	s.cmd = nil
}

// restart stops the service and starts it again on the same port and storage.
func (s *service) restart() error {
	s.stop()
	return s.start()
}

// goCmd runs the go command in dir and returns its combined output.
func goCmd(dir string, timeout time.Duration, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		err = fmt.Errorf("timed out after %v", timeout)
	}
	return string(out), err
}

// countLines counts the lines of Go code and Go tests in dir.
func countLines(dir string) (code, tests int) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		n := bytes.Count([]byte(readFile(path)), []byte("\n"))
		if strings.HasSuffix(path, "_test.go") {
			tests += n
		} else {
			code += n
		}
		return nil
	})
	return code, tests
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1500 {
		return s[:1500] + "… (truncated)"
	}
	return s
}
