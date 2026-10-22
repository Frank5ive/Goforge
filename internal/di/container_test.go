package di

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/Frank5ive/goforge/internal/domain"
)

// minimalTemplates returns a placeholder for every real template, so generation
// exercises the full file list without depending on template contents. The
// placeholder is valid Go, which also passes for non-Go files.
func minimalTemplates(t *testing.T) fstest.MapFS {
	t.Helper()

	names, err := fs.Glob(os.DirFS(filepath.Join("..", "..", "templates")), "*.tmpl")
	if err != nil || len(names) == 0 {
		t.Fatalf("Failed to list real templates: %v", err)
	}

	fsys := fstest.MapFS{}
	for _, name := range names {
		fsys[name] = &fstest.MapFile{Data: []byte("package placeholder\n")}
	}
	return fsys
}

func TestBuildContainer(t *testing.T) {
	container, err := BuildContainer(minimalTemplates(t), Options{})
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
	container, err := BuildContainer(minimalTemplates(t), Options{})
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
	container, err := BuildContainer(minimalTemplates(t), Options{DryRun: true})
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
