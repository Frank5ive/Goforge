package usecase

import (
	"fmt"
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
	WriteFileFunc      func(path string, content string) error
	CreateDirectoryFunc func(path string) error
	FileExistsFunc     func(path string) bool
	FilesWritten       map[string]string
	DirsCreated        []string
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

// Tests

func TestNewServerGeneratorUsecase(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()

	usecase := NewServerGeneratorUsecase(renderer, writer)

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

	config := &domain.ServerConfig{
		AppName:       "testapp",
		Port:          8080,
		OutputDir:     "/tmp",
		IncludeLogger: true,
		IncludeDocker: true,
		IncludeReadme: true,
	}

	err := usecase.Generate(config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify directories were created
	expectedDirs := 4 // projectPath, handlers, server, logger
	if len(writer.DirsCreated) != expectedDirs {
		t.Errorf("Expected %d directories to be created, got %d", expectedDirs, len(writer.DirsCreated))
	}

	// Verify files were written
	expectedFiles := []string{"main.go", "server.go", "health.go", "go.mod", "logger.go", 
		"middleware.go", "Dockerfile", "README.md", ".gitignore"}
	
	if len(writer.FilesWritten) != len(expectedFiles) {
		t.Errorf("Expected %d files to be written, got %d", len(expectedFiles), len(writer.FilesWritten))
	}

	// Verify templates were rendered
	expectedTemplates := []string{"main.tmpl", "server.tmpl", "handler.tmpl", "gomod.tmpl",
		"logger.tmpl", "middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl"}
	
	for _, tmpl := range expectedTemplates {
		if renderer.CallCount[tmpl] == 0 {
			t.Errorf("Expected template %s to be rendered", tmpl)
		}
	}
}

func TestServerGeneratorUsecase_Generate_WithoutOptionalFeatures(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:       "testapp",
		Port:          8080,
		OutputDir:     "/tmp",
		IncludeLogger: false,
		IncludeDocker: false,
		IncludeReadme: false,
	}

	err := usecase.Generate(config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify logger directory was not created
	expectedDirs := 3 // projectPath, handlers, server (no logger)
	if len(writer.DirsCreated) != expectedDirs {
		t.Errorf("Expected %d directories to be created, got %d", expectedDirs, len(writer.DirsCreated))
	}

	// Verify optional files were not written
	if _, exists := writer.FilesWritten["/tmp/testapp/logger/logger.go"]; exists {
		t.Error("Logger should not be generated when IncludeLogger is false")
	}

	if _, exists := writer.FilesWritten["/tmp/testapp/Dockerfile"]; exists {
		t.Error("Dockerfile should not be generated when IncludeDocker is false")
	}

	if _, exists := writer.FilesWritten["/tmp/testapp/README.md"]; exists {
		t.Error("README should not be generated when IncludeReadme is false")
	}
}

func TestServerGeneratorUsecase_Generate_InvalidConfig(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:   "", // Invalid: empty name
		Port:      8080,
		OutputDir: "/tmp",
	}

	err := usecase.Generate(config)

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	// Verify it's a generation error with validation step
	genErr, ok := err.(*domain.GenerationError)
	if !ok {
		t.Fatalf("Expected GenerationError, got %T", err)
	}

	if genErr.Step != "validation" {
		t.Errorf("Expected error step 'validation', got '%s'", genErr.Step)
	}
}

func TestServerGeneratorUsecase_Generate_ProjectExists(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()
	
	// Mock FileExists to return true
	writer.FileExistsFunc = func(path string) bool {
		return true
	}

	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:   "testapp",
		Port:      8080,
		OutputDir: "/tmp",
	}

	err := usecase.Generate(config)

	if err == nil {
		t.Fatal("Expected error when project exists, got nil")
	}

	genErr, ok := err.(*domain.GenerationError)
	if !ok {
		t.Fatalf("Expected GenerationError, got %T", err)
	}

	if genErr.Step != "directory_check" {
		t.Errorf("Expected error step 'directory_check', got '%s'", genErr.Step)
	}
}

func TestServerGeneratorUsecase_Generate_TemplateRenderError(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()

	// Mock template render to fail
	renderer.RenderFunc = func(templateName string, data interface{}) (string, error) {
		return "", fmt.Errorf("template not found")
	}

	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:   "testapp",
		Port:      8080,
		OutputDir: "/tmp",
	}

	err := usecase.Generate(config)

	if err == nil {
		t.Fatal("Expected template render error, got nil")
	}

	genErr, ok := err.(*domain.GenerationError)
	if !ok {
		t.Fatalf("Expected GenerationError, got %T", err)
	}

	if genErr.Step != "generate_main" {
		t.Errorf("Expected error at 'generate_main', got '%s'", genErr.Step)
	}
}

func TestServerGeneratorUsecase_Generate_FileWriteError(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()

	// Mock file write to fail
	writer.WriteFileFunc = func(path string, content string) error {
		return fmt.Errorf("permission denied")
	}

	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:   "testapp",
		Port:      8080,
		OutputDir: "/tmp",
	}

	err := usecase.Generate(config)

	if err == nil {
		t.Fatal("Expected file write error, got nil")
	}

	_, ok := err.(*domain.GenerationError)
	if !ok {
		t.Fatalf("Expected GenerationError, got %T", err)
	}
}

func TestServerGeneratorUsecase_Generate_DirectoryCreationError(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()

	// Mock directory creation to fail
	writer.CreateDirectoryFunc = func(path string) error {
		return fmt.Errorf("permission denied")
	}

	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:   "testapp",
		Port:      8080,
		OutputDir: "/tmp",
	}

	err := usecase.Generate(config)

	if err == nil {
		t.Fatal("Expected directory creation error, got nil")
	}

	genErr, ok := err.(*domain.GenerationError)
	if !ok {
		t.Fatalf("Expected GenerationError, got %T", err)
	}

	if genErr.Step != "create_structure" {
		t.Errorf("Expected error step 'create_structure', got '%s'", genErr.Step)
	}
}

func TestServerGeneratorUsecase_CreateProjectStructure(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := NewMockFileWriter()
	usecase := NewServerGeneratorUsecase(renderer, writer)

	config := &domain.ServerConfig{
		AppName:       "testapp",
		Port:          8080,
		OutputDir:     "/tmp",
		IncludeLogger: true,
	}

	err := usecase.createProjectStructure(config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify correct directories were created
	expectedDirs := []string{
		"/tmp/testapp",
		"/tmp/testapp/handlers",
		"/tmp/testapp/server",
		"/tmp/testapp/logger",
	}

	if len(writer.DirsCreated) != len(expectedDirs) {
		t.Errorf("Expected %d directories, got %d", len(expectedDirs), len(writer.DirsCreated))
	}

	for _, expectedDir := range expectedDirs {
		found := false
		for _, createdDir := range writer.DirsCreated {
			if createdDir == expectedDir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected directory %s to be created", expectedDir)
		}
	}
}
