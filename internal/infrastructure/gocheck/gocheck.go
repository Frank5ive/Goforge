// Package gocheck checks a Go project with gofmt, go vet, go test, and
// golangci-lint, and turns their output into structured problems.
package gocheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Frank5ive/goforge/internal/domain"
)

// maxMessage caps a problem's message, so one runaway failure can't flood the
// report (or an AI agent's context).
const maxMessage = 2000

// runFunc runs name with args in dir and returns its stdout, its stderr, and
// the error from running it (an *exec.ExitError when it exits non-zero).
type runFunc func(dir, name string, args ...string) (stdout, stderr []byte, err error)

// Checker implements domain.ProjectChecker with the go toolchain.
type Checker struct {
	run      runFunc
	lookPath func(string) (string, error)
}

// New returns a Checker that runs the real tools.
func New() *Checker {
	return &Checker{run: runCommand, lookPath: exec.LookPath}
}

func runCommand(dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Check implements domain.ProjectChecker. It runs gofmt, go vet, go test, and
// golangci-lint (when installed). Tests and lint are skipped when the code does
// not compile, since they would only repeat the compile errors.
func (c *Checker) Check(dir string, opts domain.CheckOptions) (domain.CheckReport, error) {
	goMod, err := readFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return domain.CheckReport{}, domain.WithCode(domain.CodeNotAProject, fmt.Errorf("%s is not a Go module: %w", dir, err))
	}
	if _, err := c.lookPath("go"); err != nil {
		return domain.CheckReport{}, errors.New("the go command is needed to check a project")
	}
	module := modulePath(goMod)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return domain.CheckReport{}, err
	}

	report := domain.CheckReport{Steps: []domain.CheckStep{}, Problems: []domain.CheckProblem{}}
	step := func(name string, run func() ([]domain.CheckProblem, error)) {
		start := time.Now()
		problems, err := run()
		if err != nil {
			problems = append(problems, domain.CheckProblem{Step: name, Message: truncate(err.Error())})
		}
		status := domain.StepPassed
		if len(problems) > 0 {
			status = domain.StepFailed
		}
		report.Steps = append(report.Steps, domain.CheckStep{
			Name: name, Status: status, Problems: len(problems), DurationMS: time.Since(start).Milliseconds(),
		})
		report.Problems = append(report.Problems, problems...)
	}
	skip := func(name, note string) {
		report.Steps = append(report.Steps, domain.CheckStep{Name: name, Status: domain.StepSkipped, Note: note})
	}

	step(domain.CheckFormat, func() ([]domain.CheckProblem, error) { return c.format(dir) })

	var compileFailed bool
	step(domain.CheckVet, func() ([]domain.CheckProblem, error) {
		problems, failed, err := c.vet(dir)
		compileFailed = failed
		return problems, err
	})
	if compileFailed {
		skip(domain.CheckTest, "the code does not compile; fix the vet problems first")
		skip(domain.CheckLint, "the code does not compile; fix the vet problems first")
		return report, nil
	}

	step(domain.CheckTest, func() ([]domain.CheckProblem, error) { return c.test(dir, absDir, module, opts.Race) })

	if _, err := c.lookPath("golangci-lint"); err != nil {
		skip(domain.CheckLint, "golangci-lint is not installed (https://golangci-lint.run)")
		return report, nil
	}
	step(domain.CheckLint, func() ([]domain.CheckProblem, error) { return c.lint(dir) })

	return report, nil
}

func (c *Checker) format(dir string) ([]domain.CheckProblem, error) {
	gofmt, err := c.lookPath("gofmt")
	if err != nil {
		// gofmt ships with Go, next to the go command
		goPath, _ := c.lookPath("go")
		gofmt = filepath.Join(filepath.Dir(goPath), "gofmt")
	}
	stdout, stderr, err := c.run(dir, gofmt, "-l", ".")
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w: %s", err, stderr)
	}
	return parseGofmt(stdout), nil
}

func (c *Checker) vet(dir string) ([]domain.CheckProblem, bool, error) {
	_, stderr, err := c.run(dir, "go", "vet", "./...")
	problems, compileFailed := parseVet(stderr)
	if err != nil && len(problems) == 0 {
		return nil, false, fmt.Errorf("go vet: %w: %s", err, stderr)
	}
	return problems, compileFailed, nil
}

func (c *Checker) test(dir, absDir, module string, race bool) ([]domain.CheckProblem, error) {
	args := []string{"test", "-json"}
	if race {
		args = append(args, "-race")
	}
	stdout, stderr, err := c.run(dir, "go", append(args, "./...")...)
	problems := parseTestJSON(stdout, absDir, module)
	if err != nil && len(problems) == 0 {
		return nil, fmt.Errorf("go test: %w: %s", err, stderr)
	}
	return problems, nil
}

func (c *Checker) lint(dir string) ([]domain.CheckProblem, error) {
	stdout, stderr, err := c.run(dir, "golangci-lint", "run",
		"--output.json.path=stdout", "--output.text.path=stderr", "--show-stats=false", "./...")
	problems, parseErr := parseLintJSON(stdout)
	if parseErr != nil {
		if err != nil {
			return nil, fmt.Errorf("golangci-lint: %w: %s", err, stderr)
		}
		return nil, fmt.Errorf("reading golangci-lint output: %w", parseErr)
	}
	return problems, nil
}

func parseGofmt(stdout []byte) []domain.CheckProblem {
	var problems []domain.CheckProblem
	for line := range strings.Lines(string(stdout)) {
		if file := strings.TrimSpace(line); file != "" {
			file = filepath.ToSlash(file)
			problems = append(problems, domain.CheckProblem{
				Step: domain.CheckFormat, File: file, Message: "not gofmt-formatted; run: gofmt -w " + file,
			})
		}
	}
	return problems
}

// position matches compiler and vet output: "path/file.go:12:3: message", with
// the column optional and a "vet: " prefix on type-checking errors.
var position = regexp.MustCompile(`^(vet: )?(\S+\.go):(\d+):(?:(\d+):)? (.*)$`)

// parseVet returns the problems in go vet's output, and whether any of them is
// a compile error, which vet prints with a "vet: " prefix.
func parseVet(stderr []byte) ([]domain.CheckProblem, bool) {
	var problems []domain.CheckProblem
	compileFailed := false
	seen := map[string]bool{}
	for line := range strings.Lines(string(stderr)) {
		m := position.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m == nil {
			continue
		}
		if m[1] != "" {
			compileFailed = true
		}
		// Compile errors are printed twice, with and without the prefix
		key := strings.Join(m[2:], ":")
		if seen[key] {
			continue
		}
		seen[key] = true
		p := domain.CheckProblem{Step: domain.CheckVet, File: cleanPath(m[2]), Message: m[5]}
		p.Line, _ = strconv.Atoi(m[3])
		p.Column, _ = strconv.Atoi(m[4])
		problems = append(problems, p)
	}
	return problems, compileFailed
}

// testEvent is a line of `go test -json` output.
type testEvent struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Output     string
}

// failure collects the output of a test or package that failed.
type failure struct {
	pkg, test string
	output    []string
}

// assertion matches a test's failure message: "    file_test.go:42: message".
var assertion = regexp.MustCompile(`^\s+(\S+\.go):(\d+): (.*)$`)

// parseTestJSON returns a problem for each failing test (the innermost failing
// subtest, not its parents), each package that failed outside a test, and each
// compile error in a test file. absDir and module locate the project's files.
func parseTestJSON(stdout []byte, absDir, module string) []domain.CheckProblem {
	outputs := map[[2]string][]string{}
	var failed []failure
	var buildErrors []domain.CheckProblem
	for line := range strings.Lines(string(stdout)) {
		var e testEvent
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		key := [2]string{e.Package, e.Test}
		switch e.Action {
		case "output":
			outputs[key] = append(outputs[key], e.Output)
		case "fail":
			failed = append(failed, failure{pkg: e.Package, test: e.Test, output: outputs[key]})
		case "build-output":
			if m := position.FindStringSubmatch(strings.TrimRight(e.Output, "\n")); m != nil {
				p := domain.CheckProblem{Step: domain.CheckTest, File: cleanPath(m[2]), Package: strings.Fields(e.ImportPath)[0], Message: m[5]}
				p.Line, _ = strconv.Atoi(m[3])
				p.Column, _ = strconv.Atoi(m[4])
				buildErrors = append(buildErrors, p)
			}
		}
	}

	problems := buildErrors
	for _, f := range failed {
		if f.test == "" {
			// The package failed: report it only if no test or build error explains why
			if slices.ContainsFunc(failed, func(o failure) bool { return o.pkg == f.pkg && o.test != "" }) ||
				slices.ContainsFunc(buildErrors, func(p domain.CheckProblem) bool { return p.Package == f.pkg }) {
				continue
			}
		} else if slices.ContainsFunc(failed, func(o failure) bool {
			return o.pkg == f.pkg && strings.HasPrefix(o.test, f.test+"/")
		}) {
			continue // a subtest failed; report that instead
		}
		problems = append(problems, testProblem(f, absDir, module))
	}
	return problems
}

func testProblem(f failure, absDir, module string) domain.CheckProblem {
	p := domain.CheckProblem{Step: domain.CheckTest, Package: f.pkg, Test: f.test}
	pkgDir := strings.TrimPrefix(strings.TrimPrefix(f.pkg, module), "/")

	var message []string
	panicking := false
	for _, out := range f.output {
		line := strings.TrimRight(out, "\n")
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "", strings.HasPrefix(trimmed, "=== "), strings.HasPrefix(trimmed, "--- "),
			trimmed == "FAIL", strings.HasPrefix(trimmed, "FAIL\t"), strings.HasPrefix(trimmed, "ok "):
			continue
		case strings.HasPrefix(line, "panic: "):
			panicking = true
			message = append(message, strings.TrimSuffix(line, " [recovered]"))
		case panicking:
			// The stack: keep only where the panic happened in the project
			if p.File == "" {
				p.File, p.Line = stackLocation(trimmed, absDir)
			}
		default:
			if m := assertion.FindStringSubmatch(line); m != nil && p.File == "" {
				p.File = filepath.ToSlash(filepath.Join(pkgDir, m[1]))
				p.Line, _ = strconv.Atoi(m[2])
			}
			message = append(message, trimmed)
		}
	}
	p.Message = truncate(strings.Join(message, "\n"))
	if p.Message == "" {
		p.Message = "failed"
	}
	return p
}

// stackLocation returns the project-relative file and line of a stack trace
// line such as "/abs/dir/pkg/file.go:12 +0x28", or "" if it is outside the
// project. The path may contain spaces, and the project directory and the
// frame's file may be spelled differently: through a symlink (as macOS's
// temporary directories are) or, on Windows, with short 8.3 names. Both are
// resolved before comparing.
func stackLocation(frame, absDir string) (string, int) {
	if i := strings.LastIndex(frame, " +0x"); i >= 0 {
		frame = frame[:i]
	}
	file, lineStr, ok := strings.Cut(frame, ".go:")
	if !ok {
		return "", 0
	}
	file += ".go"
	line, _ := strconv.Atoi(lineStr)

	for _, pair := range [][2]string{{absDir, file}, {resolve(absDir), resolve(file)}} {
		if rel, err := filepath.Rel(pair[0], pair[1]); err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel), line
		}
	}
	return "", 0
}

// resolve returns path with symlinks (and on Windows, short names) resolved, or
// path itself if that fails.
func resolve(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// lintOutput is the part of golangci-lint's JSON output GoForge reads.
type lintOutput struct {
	Issues []struct {
		FromLinter string
		Text       string
		Pos        struct {
			Filename     string
			Line, Column int
		}
	}
}

func parseLintJSON(stdout []byte) ([]domain.CheckProblem, error) {
	var out lintOutput
	if err := json.Unmarshal(stdout, &out); err != nil {
		return nil, err
	}
	var problems []domain.CheckProblem
	for _, issue := range out.Issues {
		problems = append(problems, domain.CheckProblem{
			Step: domain.CheckLint, File: cleanPath(issue.Pos.Filename), Line: issue.Pos.Line, Column: issue.Pos.Column,
			Linter: issue.FromLinter, Message: issue.Text,
		})
	}
	return problems, nil
}

func cleanPath(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(path), "./")
}

func truncate(s string) string {
	if len(s) <= maxMessage {
		return s
	}
	return s[:maxMessage] + "… (truncated)"
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func modulePath(goMod string) string {
	for line := range strings.Lines(goMod) {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}
