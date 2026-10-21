package di

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/Frank5ive/goforge/internal/domain"
)

// minimalTemplates returns templates that produce a small but valid project.
func minimalTemplates() fstest.MapFS {
	templates := map[string]string{
		"main.tmpl":       "package main\n\nfunc main() {}\n",
		"server.tmpl":     "package server\n",
		"middleware.tmpl": "package server\n",
		"handler.tmpl":    "package handlers\n",
		"gomod.tmpl":      "module {{.GetModuleName}}\n",
		"logger.tmpl":     "package logger\n",
		"dockerfile.tmpl": "EXPOSE {{.Port}}\n",
		"readme.tmpl":     "# {{.AppName}}\n",
		"gitignore.tmpl":  "*.log\n",
	}

	fsys := fstest.MapFS{}
	for name, content := range templates {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func TestBuildContainer(t *testing.T) {
	container, err := BuildContainer(minimalTemplates(), Options{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if container.ServerGeneratorUsecase == nil {
		t.Error("Expected ServerGeneratorUsecase to be initialized")
	}
	if container.Files == nil {
		t.Error("Expected Files to be initialized")
	}
}

func TestBuildContainer_NoTemplates(t *testing.T) {
	container, err := BuildContainer(fstest.MapFS{}, Options{})
	if err == nil {
		t.Fatal("Expected error when no templates are available, got nil")
	}

	if container != nil {
		t.Error("Expected nil container on error")
	}
}

func TestBuildContainer_GeneratesAndRecordsFiles(t *testing.T) {
	container, err := BuildContainer(minimalTemplates(), Options{})
	if err != nil {
		t.Fatalf("Failed to build container: %v", err)
	}

	config := &domain.ServerConfig{AppName: "wired", Port: 8080, OutputDir: t.TempDir()}
	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	mainPath := filepath.Join(config.GetProjectPath(), "main.go")
	if _, err := os.Stat(mainPath); err != nil {
		t.Errorf("Expected %s to exist: %v", mainPath, err)
	}

	if len(container.Files.Files()) == 0 {
		t.Error("Expected written files to be recorded")
	}
}

func TestBuildContainer_DryRun(t *testing.T) {
	container, err := BuildContainer(minimalTemplates(), Options{DryRun: true})
	if err != nil {
		t.Fatalf("Failed to build container: %v", err)
	}

	config := &domain.ServerConfig{AppName: "dry", Port: 8080, OutputDir: t.TempDir()}
	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, err := os.Stat(config.GetProjectPath()); !os.IsNotExist(err) {
		t.Error("Dry run must not create the project directory")
	}

	if len(container.Files.Files()) == 0 {
		t.Error("Expected dry run to record the files it would write")
	}
}
