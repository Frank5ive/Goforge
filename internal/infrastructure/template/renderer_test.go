package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGoTemplateRenderer(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir := t.TempDir()

	// Create a simple test template
	testTemplate := `Hello {{.Name}}!`
	err := os.WriteFile(filepath.Join(tempDir, "main.tmpl"), []byte(testTemplate), 0644)
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}

	// Create minimal template files for other required templates
	requiredTemplates := []string{
		"server.tmpl", "handler.tmpl", "gomod.tmpl", "logger.tmpl",
		"middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	}
	for _, tmplName := range requiredTemplates {
		err := os.WriteFile(filepath.Join(tempDir, tmplName), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create template %s: %v", tmplName, err)
		}
	}

	renderer, err := NewGoTemplateRenderer(tempDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if renderer == nil {
		t.Fatal("Expected renderer to be created, got nil")
	}

	if renderer.templateDir != tempDir {
		t.Errorf("Expected templateDir to be %s, got %s", tempDir, renderer.templateDir)
	}

	if len(renderer.templates) == 0 {
		t.Error("Expected templates to be loaded")
	}
}

func TestNewGoTemplateRenderer_MissingTemplate(t *testing.T) {
	// Create a temporary directory without all required templates
	tempDir := t.TempDir()

	renderer, err := NewGoTemplateRenderer(tempDir)
	if err == nil {
		t.Fatal("Expected error for missing templates, got nil")
	}

	if renderer != nil {
		t.Error("Expected renderer to be nil when templates are missing")
	}
}

func TestGoTemplateRenderer_Render(t *testing.T) {
	tempDir := t.TempDir()

	// Create test templates
	templates := map[string]string{
		"main.tmpl":       `package main\n\nfunc main() {\n\t// {{.AppName}}\n}`,
		"server.tmpl":     `Server: {{.AppName}} on port {{.Port}}`,
		"handler.tmpl":    `Handler for {{.AppName}}`,
		"gomod.tmpl":      `module {{.AppName}}`,
		"logger.tmpl":     `Logger`,
		"middleware.tmpl": `Middleware`,
		"dockerfile.tmpl": `FROM golang`,
		"readme.tmpl":     `# {{.AppName}}`,
		"gitignore.tmpl":  `*.log`,
	}

	for name, content := range templates {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create template %s: %v", name, err)
		}
	}

	renderer, err := NewGoTemplateRenderer(tempDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	// Test rendering with data
	type testData struct {
		AppName string
		Port    int
	}

	data := testData{
		AppName: "testapp",
		Port:    8080,
	}

	result, err := renderer.Render("main.tmpl", data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "testapp") {
		t.Errorf("Expected result to contain 'testapp', got: %s", result)
	}
}

func TestGoTemplateRenderer_Render_NonExistentTemplate(t *testing.T) {
	tempDir := t.TempDir()

	// Create all required templates
	for _, tmplName := range []string{
		"main.tmpl", "server.tmpl", "handler.tmpl", "gomod.tmpl",
		"logger.tmpl", "middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	} {
		err := os.WriteFile(filepath.Join(tempDir, tmplName), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}
	}

	renderer, err := NewGoTemplateRenderer(tempDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	_, err = renderer.Render("nonexistent.tmpl", nil)
	if err == nil {
		t.Fatal("Expected error for non-existent template, got nil")
	}
}

func TestGoTemplateRenderer_Render_InvalidData(t *testing.T) {
	tempDir := t.TempDir()

	// Create a template that expects a field that won't be provided
	template := `Hello {{.NonExistentField}}!`
	err := os.WriteFile(filepath.Join(tempDir, "main.tmpl"), []byte(template), 0644)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Create other required templates
	for _, tmplName := range []string{
		"server.tmpl", "handler.tmpl", "gomod.tmpl", "logger.tmpl",
		"middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	} {
		err := os.WriteFile(filepath.Join(tempDir, tmplName), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}
	}

	renderer, err := NewGoTemplateRenderer(tempDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	type emptyData struct{}
	
	// Go templates error on missing fields by default
	_, err = renderer.Render("main.tmpl", emptyData{})
	if err == nil {
		t.Fatal("Expected error for missing field, got nil")
	}
}

func TestGoTemplateRenderer_GetLoadedTemplates(t *testing.T) {
	tempDir := t.TempDir()

	// Create all required templates
	templateNames := []string{
		"main.tmpl", "server.tmpl", "handler.tmpl", "gomod.tmpl",
		"logger.tmpl", "middleware.tmpl", "dockerfile.tmpl", "readme.tmpl", "gitignore.tmpl",
	}

	for _, tmplName := range templateNames {
		err := os.WriteFile(filepath.Join(tempDir, tmplName), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}
	}

	renderer, err := NewGoTemplateRenderer(tempDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	loaded := renderer.GetLoadedTemplates()
	if len(loaded) != len(templateNames) {
		t.Errorf("Expected %d templates, got %d", len(templateNames), len(loaded))
	}
}
