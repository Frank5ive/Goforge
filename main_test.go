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
				"main.go", "go.mod", ".gitignore", "Makefile", ".golangci.yml", ".goforge.json",
				"config/config.go", "config/config_test.go",
				"server/server.go", "server/deps.go", "server/routes.go", "server/middleware.go", "server/server_test.go",
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
		if _, err := container.ResourceGeneratorUsecase.Add(r); err != nil {
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
	baseURL, _ := startServer(t, binary)

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

// startServer runs the binary on a free port until the test ends (or until the
// returned stop function is called) and returns its base URL. env adds
// environment variables such as DATABASE_URL.
func startServer(t *testing.T, binary string, env ...string) (string, func()) {
	t.Helper()

	port := freePort(t)
	var output bytes.Buffer

	cmd := exec.Command(binary)
	cmd.Env = append(append(os.Environ(), fmt.Sprintf("PORT=%d", port)), env...)
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
	stop := func() {
		_ = cmd.Process.Kill()
		<-done
	}
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			stop()
		}
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, baseURL+"/health", done, &output).Body.Close()
	return baseURL, stop
}

// databaseEngines are the engines TestAddDatabase covers, with a function that
// starts a throwaway database and returns its URL (skipping when unavailable).
var databaseEngines = []struct {
	name  string
	start func(t *testing.T) string
}{
	{domain.DatabasePostgres, startPostgres},
	{domain.DatabaseMySQL, startMySQL},
	{domain.DatabaseSQLite, func(t *testing.T) string { return filepath.Join(t.TempDir(), "shop.db") }},
}

// TestAddDatabase adds each database engine to a generated project with
// resources and checks that the project passes its tests against a real
// database, that data survives a server restart, and that /readyz reports the
// database. Server databases need Docker on Linux (skipped elsewhere); every
// engine needs network access for `go get`.
func TestAddDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end database test in short mode")
	}

	for _, engine := range databaseEngines {
		t.Run(engine.name, func(t *testing.T) {
			testAddDatabase(t, engine.name, engine.start(t))
		})
	}
}

func testAddDatabase(t *testing.T, engine, databaseURL string) {
	templates, err := fs.Sub(TemplatesFS, "templates")
	if err != nil {
		t.Fatalf("Failed to open embedded templates: %v", err)
	}
	container, err := di.BuildContainer(templates, di.Options{})
	if err != nil {
		t.Fatalf("Failed to build container: %v", err)
	}

	config := &domain.ServerConfig{AppName: "shop", ModulePath: "github.com/acme/shop", Port: 8080, OutputDir: t.TempDir(), IncludeLogger: true, IncludeCI: true}
	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		t.Fatalf("Failed to generate server: %v", err)
	}
	project := config.GetProjectPath()

	// One resource before the database, so `add db` must convert it
	product := &domain.ResourceConfig{ProjectDir: project, Name: "product", Fields: []domain.Field{
		{Name: "name", Type: domain.FieldString},
		{Name: "notes", Type: domain.FieldString, Optional: true},
		{Name: "stock", Type: domain.FieldInt},
		{Name: "price", Type: domain.FieldFloat},
		{Name: "in_stock", Type: domain.FieldBool},
		{Name: "released_at", Type: domain.FieldTime},
	}}
	if _, err := container.ResourceGeneratorUsecase.Add(product); err != nil {
		t.Fatalf("Failed to add resource: %v", err)
	}
	if _, err := container.DatabaseGeneratorUsecase.Add(&domain.DatabaseConfig{ProjectDir: project, Engine: engine}); err != nil {
		t.Fatalf("Failed to add database: %v", err)
	}

	// And one after, which must use the database from the start
	category := &domain.ResourceConfig{ProjectDir: project, Name: "category", Fields: []domain.Field{{Name: "title", Type: domain.FieldString}}}
	if _, err := container.ResourceGeneratorUsecase.Add(category); err != nil {
		t.Fatalf("Failed to add resource after the database: %v", err)
	}
	routes, _ := os.ReadFile(filepath.Join(project, "server", "routes.go"))
	if strings.Contains(string(routes), "NewMemoryRepository") {
		t.Errorf("a resource is still stored in memory:\n%s", routes)
	}

	assertGofmtClean(t, project)
	runGo(t, project, "vet", "./...")
	runGoEnv(t, project, []string{"TEST_DATABASE_URL=" + databaseURL}, "test", "-count=1", "./...")

	binary := filepath.Join(t.TempDir(), "shop")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	runGo(t, project, "build", "-o", binary, ".")

	baseURL, stop := startServer(t, binary, "DATABASE_URL="+databaseURL)
	body := `{"name":"Gopher plush","notes":"","stock":3,"price":19.5,"in_stock":true,"released_at":"2026-03-01T00:00:00Z"}`
	if status, _, resp := request(t, http.MethodPost, baseURL+"/products", body); status != http.StatusCreated {
		t.Fatalf("POST /products = %d %s", status, resp)
	}
	if status, _, resp := request(t, http.MethodPost, baseURL+"/categories", `{"title":"Toys"}`); status != http.StatusCreated {
		t.Fatalf("POST /categories = %d %s", status, resp)
	}
	if status, _, _ := request(t, http.MethodGet, baseURL+"/readyz", ""); status != http.StatusOK {
		t.Errorf("GET /readyz = %d, want 200 with the database up", status)
	}
	stop()

	// Data is in the database, so it survives a restart
	baseURL, _ = startServer(t, binary, "DATABASE_URL="+databaseURL)
	var page struct {
		Total int `json:"total"`
	}
	for _, path := range []string{"/products", "/categories"} {
		if status, _, resp := request(t, http.MethodGet, baseURL+path, ""); status != http.StatusOK || json.Unmarshal(resp, &page) != nil || page.Total < 1 {
			t.Errorf("GET %s after restart = %d %s, want the stored item", path, status, resp)
		}
	}
}

// startPostgres starts a throwaway PostgreSQL container and returns its URL.
func startPostgres(t *testing.T) string {
	hostPort := startContainer(t, "postgres:18-alpine", "5432/tcp",
		[]string{"POSTGRES_USER=app", "POSTGRES_PASSWORD=app", "POSTGRES_DB=shop"},
		[]string{"pg_isready", "-U", "app", "-d", "shop", "-h", "127.0.0.1"})
	return fmt.Sprintf("postgres://app:app@%s/shop?sslmode=disable", hostPort)
}

// startMySQL starts a throwaway MySQL container and returns its URL.
func startMySQL(t *testing.T) string {
	// Ping over TCP: during initialization MySQL runs a temporary server that only
	// accepts socket connections, so this succeeds only once it is really ready
	hostPort := startContainer(t, "mysql:8.4", "3306/tcp",
		[]string{"MYSQL_ROOT_PASSWORD=root", "MYSQL_DATABASE=shop", "MYSQL_USER=app", "MYSQL_PASSWORD=app"},
		[]string{"mysqladmin", "ping", "-h", "127.0.0.1", "-uapp", "-papp", "--silent"})
	return fmt.Sprintf("app:app@tcp(%s)/shop", hostPort)
}

// startContainer runs image with env, publishing port on a random local port,
// waits until readyCmd succeeds inside the container, and returns the host:port.
// It skips the test unless Docker is available on Linux.
func startContainer(t *testing.T, image, port string, env, readyCmd []string) string {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("database end-to-end tests run on Linux only")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("Docker is not available")
	}

	args := []string{"run", "-d", "--rm", "-p", "127.0.0.1::" + strings.TrimSuffix(port, "/tcp")}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	out, err := exec.Command("docker", append(args, image)...).Output()
	if err != nil {
		t.Fatalf("starting %s: %v", image, err)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })

	out, err = exec.Command("docker", "port", id, port).Output()
	if err != nil {
		t.Fatalf("finding the %s port: %v", image, err)
	}
	hostPort := strings.TrimSpace(strings.Split(string(out), "\n")[0])

	deadline := time.Now().Add(120 * time.Second)
	for exec.Command("docker", append([]string{"exec", id}, readyCmd...)...).Run() != nil {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not become ready within 120s", image)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return hostPort
}

// runGoEnv is runGo with extra environment variables.
func runGoEnv(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GOWORK=off"), env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s failed: %v\nOutput:\n%s", strings.Join(args, " "), err, output)
	}
}
