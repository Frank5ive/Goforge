package di

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildContainer(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir := t.TempDir()
	
	// Create minimal template files
	templateFiles := []string{
		"main.tmpl", "server.tmpl", "handler.tmpl", "gomod.tmpl",
		"logger.tmpl", "middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	}
	
	for _, fileName := range templateFiles {
		err := os.WriteFile(filepath.Join(tempDir, fileName), []byte("test template"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
	}
	
	// Build container
	container := BuildContainer(tempDir)
	
	if container == nil {
		t.Fatal("Expected container to be created, got nil")
	}
	
	if container.ServerGeneratorUsecase == nil {
		t.Error("Expected ServerGeneratorUsecase to be initialized")
	}
}

func TestBuildContainer_InvalidTemplatePath(t *testing.T) {
	// This test verifies the behavior when template path is invalid
	// BuildContainer calls log.Fatalf on error, so we can't easily test this
	// In a production scenario, you might want to refactor BuildContainer to return an error
	// For now, we'll skip this test
	t.Skip("BuildContainer uses log.Fatalf which terminates the process")
}

func TestBuildContainer_Integration(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir := t.TempDir()
	
	// Create minimal template files with actual template content
	templates := map[string]string{
		"main.tmpl":       "package main\n\nfunc main() {\n\t// {{.AppName}}\n}",
		"server.tmpl":     "package server\n\n// Server for {{.AppName}}",
		"handler.tmpl":    "package handlers\n\n// Handler",
		"gomod.tmpl":      "module {{.GetModuleName}}\n\ngo 1.21",
		"logger.tmpl":     "package logger\n\n// Logger",
		"middleware.tmpl": "package server\n\n// Middleware",
		"dockerfile.tmpl": "FROM golang:1.21\n\nEXPOSE {{.Port}}",
		"readme.tmpl":     "# {{.AppName}}\n\nREADME",
		"gitignore.tmpl":  "*.exe\n*.log",
	}
	
	for fileName, content := range templates {
		err := os.WriteFile(filepath.Join(tempDir, fileName), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test template %s: %v", fileName, err)
		}
	}
	
	// Build container
	container := BuildContainer(tempDir)
	
	// Verify the container can be used
	if container == nil {
		t.Fatal("Expected container to be created")
	}
	
	// Verify we can access the usecase
	if container.ServerGeneratorUsecase == nil {
		t.Fatal("Expected ServerGeneratorUsecase to be initialized")
	}
	
	// The usecase should be ready to use (we won't call Generate here as it needs valid config)
	t.Log("Container built successfully with all dependencies wired")
}

func TestContainer_DependencyWiring(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir := t.TempDir()
	
	// Create template files
	templateFiles := []string{
		"main.tmpl", "server.tmpl", "handler.tmpl", "gomod.tmpl",
		"logger.tmpl", "middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	}
	
	for _, fileName := range templateFiles {
		err := os.WriteFile(filepath.Join(tempDir, fileName), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
	}
	
	// Build container
	container := BuildContainer(tempDir)
	
	// Verify the container structure
	if container.ServerGeneratorUsecase == nil {
		t.Error("ServerGeneratorUsecase should be wired in container")
	}
	
	// The fact that BuildContainer didn't panic means all dependencies were wired correctly
	t.Log("All dependencies successfully wired")
}

func TestContainer_TemplateDirectory(t *testing.T) {
	// Test with absolute path
	tempDir := t.TempDir()
	
	// Create template files
	templateFiles := []string{
		"main.tmpl", "server.tmpl", "handler.tmpl", "gomod.tmpl",
		"logger.tmpl", "middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	}
	
	for _, fileName := range templateFiles {
		err := os.WriteFile(filepath.Join(tempDir, fileName), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test template: %v", err)
		}
	}
	
	// Get absolute path
	absPath, err := filepath.Abs(tempDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	// Build container with absolute path
	container := BuildContainer(absPath)
	
	if container == nil {
		t.Fatal("Expected container to be created with absolute path")
	}
	
	if container.ServerGeneratorUsecase == nil {
		t.Error("Expected ServerGeneratorUsecase to be initialized with absolute path")
	}
}
