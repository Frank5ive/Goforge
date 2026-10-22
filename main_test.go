package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

// TestGeneratedProjects generates a project for every combination of optional
// features using the real embedded templates (the same ones shipped in the binary),
// then verifies each project is gofmt-clean, passes go vet and its own tests, builds,
// serves /health, and shuts down gracefully on SIGTERM.
func TestGeneratedProjects(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end generation test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found in PATH")
	}

	templates, err := fs.Sub(TemplatesFS, "templates")
	if err != nil {
		t.Fatalf("Failed to open embedded templates: %v", err)
	}

	// Every combination of the four optional features
	for mask := 0; mask < 16; mask++ {
		includeLogger, includeDocker := mask&1 != 0, mask&2 != 0
		includeReadme, includeCI := mask&4 != 0, mask&8 != 0

		name := fmt.Sprintf("logger=%t/docker=%t/readme=%t/ci=%t", includeLogger, includeDocker, includeReadme, includeCI)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Alternate between an explicit module path and the default (the app name)
			modulePath := ""
			if includeReadme {
				modulePath = "github.com/acme/test-api"
			}

			config := &domain.ServerConfig{
				AppName:       "test-api",
				ModulePath:    modulePath,
				Port:          8080,
				OutputDir:     t.TempDir(),
				IncludeLogger: includeLogger,
				IncludeDocker: includeDocker,
				IncludeReadme: includeReadme,
				IncludeCI:     includeCI,
			}

			// Each subtest gets its own container, since they run in parallel
			container, err := di.BuildContainer(templates, di.Options{})
			if err != nil {
				t.Fatalf("Failed to build container: %v", err)
			}
			if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
				t.Fatalf("Failed to generate server: %v", err)
			}

			projectPath := config.GetProjectPath()
			for _, file := range []string{
				"main.go", "go.mod", ".gitignore", "Makefile", ".golangci.yml",
				"config/config.go", "config/config_test.go",
				"server/server.go", "server/routes.go", "server/middleware.go", "server/server_test.go",
				"handlers/health.go", "handlers/respond.go", "handlers/health_test.go", "handlers/respond_test.go",
			} {
				assertFileExists(t, filepath.Join(projectPath, file), true)
			}
			assertFileExists(t, filepath.Join(projectPath, "logger", "logger.go"), includeLogger)
			assertFileExists(t, filepath.Join(projectPath, "logger", "logger_test.go"), includeLogger)
			assertFileExists(t, filepath.Join(projectPath, "Dockerfile"), includeDocker)
			assertFileExists(t, filepath.Join(projectPath, ".dockerignore"), includeDocker)
			assertFileExists(t, filepath.Join(projectPath, "README.md"), includeReadme)
			assertFileExists(t, filepath.Join(projectPath, ".github", "workflows", "ci.yml"), includeCI)

			assertGofmtClean(t, projectPath)
			runGo(t, projectPath, "vet", "./...")
			runGo(t, projectPath, "test", "./...")

			binary := filepath.Join(t.TempDir(), "server-under-test")
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			runGo(t, projectPath, "build", "-o", binary, ".")

			assertServerLifecycle(t, binary, config.AppName)
		})
	}
}

// assertServerLifecycle starts the built server on a free port (via the PORT
// environment variable), checks /health, then sends SIGTERM and expects a clean exit.
func assertServerLifecycle(t *testing.T, binary, appName string) {
	t.Helper()

	port := freePort(t)
	var output bytes.Buffer

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start generated server: %v", err)
	}

	// done is closed once the process exits; waitErr is safe to read after that
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			cmd.Process.Kill()
			<-done
		}
	})

	resp := waitForHealth(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), done, &output)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}

	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode /health response: %v", err)
	}
	if body.Status != "healthy" || body.Service != appName {
		t.Errorf("Unexpected /health body: %+v", body)
	}

	// Signal-based shutdown is not supported on Windows
	if runtime.GOOS == "windows" {
		return
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	select {
	case <-done:
		if waitErr != nil {
			t.Fatalf("Expected clean exit after SIGTERM, got %v\nServer output:\n%s", waitErr, output.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("Server did not exit within 10s of SIGTERM\nServer output:\n%s", output.String())
	}

	if !strings.Contains(output.String(), "server exited gracefully") {
		t.Errorf("Expected graceful shutdown message\nServer output:\n%s", output.String())
	}
}

// waitForHealth polls the health endpoint until it responds, the server exits, or 10s pass.
func waitForHealth(t *testing.T, url string, done <-chan struct{}, output *bytes.Buffer) *http.Response {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			return resp
		}

		select {
		case <-done:
			t.Fatalf("Server exited before becoming ready\nServer output:\n%s", output.String())
		default:
		}

		if time.Now().After(deadline) {
			t.Fatalf("Server not ready after 10s: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// freePort asks the OS for an unused TCP port.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find a free port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

// runGo runs a go subcommand in dir and fails the test with its output on error.
func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s failed: %v\nOutput:\n%s", strings.Join(args, " "), err, output)
	}
}

// assertGofmtClean fails if any generated .go file is not gofmt-formatted.
func assertGofmtClean(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		formatted, err := format.Source(src)
		if err != nil {
			t.Errorf("%s does not parse: %v", path, err)
			return nil
		}
		if !bytes.Equal(src, formatted) {
			t.Errorf("%s is not gofmt-formatted", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk generated project: %v", err)
	}
}

// assertFileExists checks whether a file exists, matching the expectation.
func assertFileExists(t *testing.T, path string, want bool) {
	t.Helper()

	_, err := os.Stat(path)
	if exists := err == nil; exists != want {
		t.Errorf("Expected %s to exist=%t, got exist=%t", path, want, exists)
	}
}

// TestAddResource generates a project, adds resources covering every field type
// with `goforge add resource`'s usecase, and checks the result is gofmt-clean,
// passes go vet and its own tests, builds, and serves working CRUD endpoints.
func TestAddResource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end resource test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found in PATH")
	}

	templates, err := fs.Sub(TemplatesFS, "templates")
	if err != nil {
		t.Fatalf("Failed to open embedded templates: %v", err)
	}
	container, err := di.BuildContainer(templates, di.Options{})
	if err != nil {
		t.Fatalf("Failed to build container: %v", err)
	}

	config := &domain.ServerConfig{
		AppName:       "shop",
		ModulePath:    "github.com/acme/shop",
		Port:          8080,
		OutputDir:     t.TempDir(),
		IncludeLogger: true,
	}
	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		t.Fatalf("Failed to generate server: %v", err)
	}
	project := config.GetProjectPath()

	resources := []*domain.ResourceConfig{
		{Name: "product", Fields: []domain.Field{
			{Name: "name", Type: domain.FieldString},
			{Name: "image_url", Type: domain.FieldString, Optional: true},
			{Name: "stock", Type: domain.FieldInt},
			{Name: "price", Type: domain.FieldFloat},
			{Name: "in_stock", Type: domain.FieldBool},
			{Name: "released_at", Type: domain.FieldTime},
		}},
		// No required string fields, so Validate has no rules
		{Name: "counter", Fields: []domain.Field{{Name: "count", Type: domain.FieldInt}}},
		// Irregular plural
		{Name: "person", Plural: "people", Fields: []domain.Field{{Name: "name", Type: domain.FieldString}}},
	}
	for _, r := range resources {
		r.ProjectDir = project
		if err := container.ResourceGeneratorUsecase.Add(r); err != nil {
			t.Fatalf("Failed to add resource %s: %v", r.Name, err)
		}
	}

	assertGofmtClean(t, project)
	runGo(t, project, "vet", "./...")
	runGo(t, project, "test", "./...")

	binary := filepath.Join(t.TempDir(), "shop")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	runGo(t, project, "build", "-o", binary, ".")
	baseURL := startServer(t, binary)

	// Every endpoint works straight after `add resource` and a build: no manual wiring
	product := `{"name":"Gopher plush","image_url":"","stock":3,"price":19.5,"in_stock":true,"released_at":"2026-03-01T00:00:00Z"}`
	status, header, _ := request(t, http.MethodPost, baseURL+"/products", product)
	if status != http.StatusCreated {
		t.Fatalf("POST /products = %d, want %d", status, http.StatusCreated)
	}
	item := baseURL + header.Get("Location")

	var got struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}
	if status, _, body := request(t, http.MethodGet, item, ""); status != http.StatusOK || json.Unmarshal(body, &got) != nil || got.Name != "Gopher plush" {
		t.Errorf("GET created product = %d %s", status, body)
	}

	request(t, http.MethodPost, baseURL+"/products", product)
	var page struct {
		Items []json.RawMessage `json:"items"`
		Total int               `json:"total"`
	}
	if status, _, body := request(t, http.MethodGet, baseURL+"/products?limit=1&offset=1", ""); status != http.StatusOK || json.Unmarshal(body, &page) != nil || len(page.Items) != 1 || page.Total != 2 {
		t.Errorf("GET /products?limit=1&offset=1 = %d %s, want 1 of 2 items", status, body)
	}

	replaced := strings.Replace(product, `"Gopher plush"`, `"Gopher plush XL"`, 1)
	if status, _, body := request(t, http.MethodPut, item, replaced); status != http.StatusOK {
		t.Errorf("PUT product = %d %s", status, body)
	}

	if status, _, body := request(t, http.MethodPatch, item, `{"price": 24.5}`); status != http.StatusOK || json.Unmarshal(body, &got) != nil || got.Price != 24.5 || got.Name != "Gopher plush XL" {
		t.Errorf("PATCH product = %d %s, want only the price changed", status, body)
	}

	if status, _, _ := request(t, http.MethodDelete, item, ""); status != http.StatusNoContent {
		t.Errorf("DELETE product = %d, want %d", status, http.StatusNoContent)
	}
	if status, _, _ := request(t, http.MethodGet, item, ""); status != http.StatusNotFound {
		t.Errorf("GET deleted product = %d, want %d", status, http.StatusNotFound)
	}

	// The other resources are registered alongside it
	for _, path := range []string{"/people", "/counters"} {
		if status, _, _ := request(t, http.MethodGet, baseURL+path, ""); status != http.StatusOK {
			t.Errorf("GET %s = %d, want %d", path, status, http.StatusOK)
		}
	}
}

// request sends an HTTP request with an optional JSON body and returns the
// status, headers, and body of the response.
func request(t *testing.T, method, url, body string) (int, http.Header, []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building %s %s: %v", method, url, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s %s response: %v", method, url, err)
	}
	return resp.StatusCode, resp.Header, respBody
}

// startServer runs the binary on a free port until the test ends and returns its base URL.
func startServer(t *testing.T, binary string) string {
	t.Helper()

	port := freePort(t)
	var output bytes.Buffer

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, baseURL+"/health", done, &output).Body.Close()
	return baseURL
}
