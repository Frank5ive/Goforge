package template

import (
	"strings"
	"testing"
	"testing/fstest"
)

// templateFS builds an in-memory filesystem from template name to content.
func templateFS(templates map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, content := range templates {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func TestNewGoTemplateRenderer(t *testing.T) {
	fsys := templateFS(map[string]string{
		"main.tmpl":           `Hello {{.Name}}!`,
		"server.tmpl":         `test`,
		"resource/model.tmpl": `nested`,
		"notes.txt":           `not a template`,
	})

	renderer, err := NewGoTemplateRenderer(fsys)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	got := renderer.GetLoadedTemplates()
	want := []string{"main.tmpl", "resource/model.tmpl", "server.tmpl"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected templates %v, got %v", want, got)
	}
}

func TestNewGoTemplateRenderer_NoTemplates(t *testing.T) {
	renderer, err := NewGoTemplateRenderer(fstest.MapFS{})
	if err == nil {
		t.Fatal("Expected error for empty template directory, got nil")
	}

	if renderer != nil {
		t.Error("Expected renderer to be nil when no templates are found")
	}
}

func TestNewGoTemplateRenderer_InvalidSyntax(t *testing.T) {
	fsys := templateFS(map[string]string{
		"broken.tmpl": `{{if .AppName}}never closed`,
	})

	_, err := NewGoTemplateRenderer(fsys)
	if err == nil {
		t.Fatal("Expected parse error, got nil")
	}

	if !strings.Contains(err.Error(), "broken.tmpl") {
		t.Errorf("Expected error to name the broken template, got: %v", err)
	}
}

func TestGoTemplateRenderer_Render(t *testing.T) {
	renderer, err := NewGoTemplateRenderer(templateFS(map[string]string{
		"main.tmpl": "package main\n\nfunc main() {\n\t// {{.AppName}} on {{.Port}}\n}",
	}))
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	data := struct {
		AppName string
		Port    int
	}{AppName: "testapp", Port: 8080}

	result, err := renderer.Render("main.tmpl", data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "testapp on 8080") {
		t.Errorf("Expected rendered data in result, got: %s", result)
	}
}

func TestGoTemplateRenderer_Render_NonExistentTemplate(t *testing.T) {
	renderer, err := NewGoTemplateRenderer(templateFS(map[string]string{"main.tmpl": "test"}))
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	_, err = renderer.Render("nonexistent.tmpl", nil)
	if err == nil {
		t.Fatal("Expected error for non-existent template, got nil")
	}
}

func TestGoTemplateRenderer_Render_InvalidData(t *testing.T) {
	renderer, err := NewGoTemplateRenderer(templateFS(map[string]string{
		"main.tmpl": `Hello {{.NonExistentField}}!`,
	}))
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	// Go templates error on missing struct fields
	_, err = renderer.Render("main.tmpl", struct{}{})
	if err == nil {
		t.Fatal("Expected error for missing field, got nil")
	}
}
