package usecase

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

// Mock implementations for testing

type MockTemplateRenderer struct {
	RenderFunc func(templateName string, data interface{}) (string, error)
	CallCount  map[string]int
}

func NewMockTemplateRenderer() *MockTemplateRenderer {
	return &MockTemplateRenderer{
		CallCount: make(map[string]int),
	}
}

func (m *MockTemplateRenderer) Render(templateName string, data interface{}) (string, error) {
	m.CallCount[templateName]++
	if m.RenderFunc != nil {
		return m.RenderFunc(templateName, data)
	}
	return fmt.Sprintf("// Generated from %s", templateName), nil
}

type MockFileWriter struct {
	WriteFileFunc       func(path string, content string) error
	CreateDirectoryFunc func(path string) error
	FileExistsFunc      func(path string) bool
	RemoveAllFunc       func(path string) error
	FilesWritten        map[string]string
	DirsCreated         []string
	PathsRemoved        []string
}

func NewMockFileWriter() *MockFileWriter {
	return &MockFileWriter{
		FilesWritten: make(map[string]string),
		DirsCreated:  make([]string, 0),
	}
}

func (m *MockFileWriter) WriteFile(path string, content string) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(path, content)
	}
	m.FilesWritten[path] = content
	return nil
}

func (m *MockFileWriter) CreateDirectory(path string) error {
	if m.CreateDirectoryFunc != nil {
		return m.CreateDirectoryFunc(path)
	}
	m.DirsCreated = append(m.DirsCreated, path)
	return nil
}

func (m *MockFileWriter) FileExists(path string) bool {
	if m.FileExistsFunc != nil {
		return m.FileExistsFunc(path)
	}
	return false
}

func (m *MockFileWriter) RemoveAll(path string) error {
	m.PathsRemoved = append(m.PathsRemoved, path)
	if m.RemoveAllFunc != nil {
		return m.RemoveAllFunc(path)
	}
	return nil
}

func testConfig() *domain.ServerConfig {
	return &domain.ServerConfig{
		AppName:   "testapp",
		Port:      8080,
		OutputDir: "/tmp",
	}
}

// projectFilePath returns the absolute path of a generated file in the test project.
func projectFilePath(relative string) string {
	return filepath.Join("/tmp", "testapp", filepath.FromSlash(relative))
}

// assertGenerationStep checks that err is a GenerationError at the given step.
func assertGenerationStep(t *testing.T, err error, step string) *domain.GenerationError {
	t.Helper()

	var genErr *domain.GenerationError
	if !errors.As(err, &genErr) {
		t.Fatalf("Expected GenerationError, got %T: %v", err, err)
	}
	if genErr.Step != step {
		t.Errorf("Expected error step '%s', got '%s'", step, genErr.Step)
	}
	return genErr
}

// Tests

func TestNewServerGeneratorUsecase(t *testing.T) {
	usecase := NewServerGeneratorUsecase(NewMockTemplateRenderer(), NewMockFileWriter())

	if usecase == nil {
		t.Fatal("Expected usecase to be created, got nil")
	}
	if usecase.templateRenderer == nil {
		t.Error("Expected templateRenderer to be set")
	}
	if usecase.fileWriter == nil {
		t.Error("Expected fileWriter to be set")
	}
}

func TestServerGeneratorUsecase_Generate_Success(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := testConfig()
	config.IncludeLogger = true
	config.IncludeDocker = true
	config.IncludeReadme = true
	config.IncludeCI = true

	if err := usecase.Generate(config); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(writer.DirsCreated) != 1 || writer.DirsCreated[0] != projectFilePath("") {
		t.Errorf("Expected only the project directory to be created, got %v", writer.DirsCreated)
	}

	if len(writer.FilesWritten) != len(projectFiles) {
		t.Errorf("Expected %d files to be written, got %d", len(projectFiles), len(writer.FilesWritten))
	}

	for _, file := range projectFiles {
		if _, ok := writer.FilesWritten[projectFilePath(file.path)]; !ok {
			t.Errorf("Expected %s to be written", file.path)
		}
		if renderer.CallCount[file.template] != 1 {
			t.Errorf("Expected template %s to be rendered once, got %d", file.template, renderer.CallCount[file.template])
		}
	}

	if len(writer.PathsRemoved) != 0 {
		t.Errorf("Expected no cleanup on success, got %v", writer.PathsRemoved)
	}
}

func TestServerGeneratorUsecase_Generate_WithoutOptionalFeatures(t *testing.T) {
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(NewMockTemplateRenderer(), writer)

	if err := usecase.Generate(testConfig()); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	for _, optional := range []string{"logger/logger.go", "logger/logger_test.go", "Dockerfile", ".dockerignore", "README.md", ".github/workflows/ci.yml"} {
		if _, exists := writer.FilesWritten[projectFilePath(optional)]; exists {
			t.Errorf("%s should not be generated when its option is disabled", optional)
		}
	}

	for _, required := range []string{"main.go", "go.mod", "server/server.go", "handlers/health.go", ".gitignore"} {
		if _, exists := writer.FilesWritten[projectFilePath(required)]; !exists {
			t.Errorf("Expected %s to be generated", required)
		}
	}
}

func TestServerGeneratorUsecase_Generate_InvalidConfig(t *testing.T) {
	usecase := NewServerGeneratorUsecase(NewMockTemplateRenderer(), NewMockFileWriter())

	config := testConfig()
	config.AppName = "" // Invalid: empty name

	err := usecase.Generate(config)
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	assertGenerationStep(t, err, "validation")
}

func TestServerGeneratorUsecase_Generate_ProjectExists(t *testing.T) {
	writer := NewMockFileWriter()
	writer.FileExistsFunc = func(path string) bool { return true }
	usecase := NewServerGeneratorUsecase(NewMockTemplateRenderer(), writer)

	err := usecase.Generate(testConfig())
	if err == nil {
		t.Fatal("Expected error when project exists, got nil")
	}

	assertGenerationStep(t, err, "directory_check")

	if len(writer.PathsRemoved) != 0 {
		t.Errorf("Must never remove a directory that already existed, removed %v", writer.PathsRemoved)
	}
}

func TestServerGeneratorUsecase_Generate_TemplateRenderError(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	renderer.RenderFunc = func(templateName string, data interface{}) (string, error) {
		return "", fmt.Errorf("template not found")
	}
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	err := usecase.Generate(testConfig())
	if err == nil {
		t.Fatal("Expected template render error, got nil")
	}

	genErr := assertGenerationStep(t, err, "generate_files")
	if !strings.Contains(genErr.Message, "main.go") {
		t.Errorf("Expected error to name the failing file, got: %s", genErr.Message)
	}
}

func TestServerGeneratorUsecase_Generate_CleansUpOnFailure(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	renderer.RenderFunc = func(templateName string, data interface{}) (string, error) {
		if templateName == "handler.tmpl" {
			return "", fmt.Errorf("boom")
		}
		return "// ok", nil
	}
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	if err := usecase.Generate(testConfig()); err == nil {
		t.Fatal("Expected error, got nil")
	}

	if len(writer.PathsRemoved) != 1 || writer.PathsRemoved[0] != projectFilePath("") {
		t.Errorf("Expected the project directory to be removed, got %v", writer.PathsRemoved)
	}
}

func TestServerGeneratorUsecase_Generate_ReportsCleanupFailure(t *testing.T) {
	writer := NewMockFileWriter()
	writer.WriteFileFunc = func(path string, content string) error {
		return fmt.Errorf("disk full")
	}
	writer.RemoveAllFunc = func(path string) error {
		return fmt.Errorf("permission denied")
	}
	usecase := NewServerGeneratorUsecase(NewMockTemplateRenderer(), writer)

	err := usecase.Generate(testConfig())
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "cleanup") || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected cleanup failure in error, got: %v", err)
	}
}

func TestServerGeneratorUsecase_Generate_InvalidGoOutput(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	// Mock template render to produce Go that does not parse
	renderer.RenderFunc = func(templateName string, data interface{}) (string, error) {
		return "package main\nfunc main() {", nil
	}
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	err := usecase.Generate(testConfig())
	if err == nil {
		t.Fatal("Expected error for invalid Go output, got nil")
	}

	genErr := assertGenerationStep(t, err, "generate_files")
	if !strings.Contains(genErr.Error(), "not valid Go") {
		t.Errorf("Expected 'not valid Go' in error, got: %v", genErr)
	}

	if len(writer.FilesWritten) != 0 {
		t.Errorf("Expected no files to be written, got %d", len(writer.FilesWritten))
	}
}

func TestServerGeneratorUsecase_Generate_FormatsGoOutput(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	renderer.RenderFunc = func(templateName string, data interface{}) (string, error) {
		return "package main\nfunc   main( ) {\n}\n", nil
	}
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	if err := usecase.Generate(testConfig()); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if got := writer.FilesWritten[projectFilePath("main.go")]; got != "package main\n\nfunc main() {\n}\n" {
		t.Errorf("Expected gofmt-formatted main.go, got %q", got)
	}

	// Non-Go files are written exactly as rendered
	if got := writer.FilesWritten[projectFilePath("go.mod")]; got != "package main\nfunc   main( ) {\n}\n" {
		t.Errorf("Expected go.mod to be written unchanged, got %q", got)
	}
}

func TestServerGeneratorUsecase_Generate_DirectoryCreationError(t *testing.T) {
	writer := NewMockFileWriter()
	writer.CreateDirectoryFunc = func(path string) error {
		return fmt.Errorf("permission denied")
	}
	usecase := NewServerGeneratorUsecase(NewMockTemplateRenderer(), writer)

	err := usecase.Generate(testConfig())
	if err == nil {
		t.Fatal("Expected directory creation error, got nil")
	}

	assertGenerationStep(t, err, "create_structure")
}
