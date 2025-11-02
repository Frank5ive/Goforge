package main_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

// TestServerGeneratorIntegration performs an end-to-end test of the server generation process.
func TestServerGeneratorIntegration(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "server-generator-integration-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a templates subdirectory
	templateDir := filepath.Join(tempDir, "templates")
	if err := os.Mkdir(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	// Create dummy template files
	createDummyTemplates(t, templateDir)

	// Build the DI container
	container := di.BuildContainer(templateDir)

	// Define the server configuration for the test
	config := &domain.ServerConfig{
		AppName:       "testapp",
		Port:          8081,
		OutputDir:     tempDir,
		IncludeLogger: true,
		IncludeDocker: true,
		IncludeReadme: true,
	}

	// Generate the server
	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		t.Fatalf("Failed to generate server: %v", err)
	}

	// Verify that the project files were created
	projectPath := config.GetProjectPath()
	assertFileExists(t, filepath.Join(projectPath, "main.go"))
	assertFileExists(t, filepath.Join(projectPath, "go.mod"))
	assertFileExists(t, filepath.Join(projectPath, "server", "server.go"))
	assertFileExists(t, filepath.Join(projectPath, "handlers", "health.go"))

	// Compile the generated server
	cmd := exec.Command("go", "build", "-o", "testapp_server", ".")
	cmd.Dir = projectPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build generated server: %v\nOutput: %s", err, string(output))
	}

	// Run the generated server
	serverCmd := exec.Command("./testapp_server")
	serverCmd.Dir = projectPath
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start generated server: %v", err)
	}
	defer serverCmd.Process.Kill()

	// Wait for the server to start
	time.Sleep(2 * time.Second)

	// Make a request to the /health endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", config.Port))
	if err != nil {
		t.Fatalf("Failed to make request to /health endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

// createDummyTemplates creates minimal template files for the test.
func createDummyTemplates(t *testing.T, templateDir string) {
	templates := map[string]string{
		"main.tmpl":       `package main; import ("fmt"; "net/http"); func main() { http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "OK") }); http.ListenAndServe(":8081", nil) }`,
		"server.tmpl":     `package server; type Server struct{}`,
		"handler.tmpl":    `package handlers; import "net/http"; func HealthHandler(w http.ResponseWriter, r *http.Request) {}`,
		"gomod.tmpl":      `module {{.AppName}}`,
		"logger.tmpl":     `package logger; func New() {}`,
		"middleware.tmpl": `package server; import "net/http"; func LoggingMiddleware(next http.Handler) http.Handler { return next }`,
		"dockerfile.tmpl": `FROM golang:1.19`,
		"readme.tmpl":     `# {{.AppName}}`,
		"gitignore.tmpl":  `testapp_server`,
	}

	for name, content := range templates {
		filePath := filepath.Join(templateDir, name)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write dummy template %s: %v", name, err)
		}
	}
}

// assertFileExists checks if a file exists and fails the test if it doesn't.
func assertFileExists(t *testing.T, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist, but it doesn't: %s", path)
	}
}
