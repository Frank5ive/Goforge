package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"text/template"

	"github.com/Frank5ive/goforge/internal/domain"
)

// GoTemplateRenderer implements domain.TemplateRenderer using Go's text/template package.
type GoTemplateRenderer struct {
	templates map[string]*template.Template
}

// NewGoTemplateRenderer creates a renderer from every *.tmpl file in fsys, including
// subdirectories. Templates are named by their slash-separated path, e.g.
// "main.tmpl" or "resource/model.tmpl". Pass os.DirFS(dir) for templates on disk,
// or fs.Sub(embedded, "templates") for embedded ones. Templates are parsed once up
// front, so syntax errors fail fast.
func NewGoTemplateRenderer(fsys fs.FS) (*GoTemplateRenderer, error) {
	var names []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".tmpl") {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return nil, domain.NewTemplateError("*.tmpl", "failed to list templates", err)
	}
	if len(names) == 0 {
		return nil, domain.NewTemplateError("*.tmpl", "no templates found", nil)
	}

	renderer := &GoTemplateRenderer{
		templates: make(map[string]*template.Template, len(names)),
	}

	for _, name := range names {
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, domain.NewTemplateError(name, "failed to read template", err)
		}

		tmpl, err := template.New(name).Parse(string(content))
		if err != nil {
			return nil, domain.NewTemplateError(name, fmt.Sprintf("failed to parse template %s", name), err)
		}

		renderer.templates[name] = tmpl
	}

	return renderer, nil
}

// Render implements domain.TemplateRenderer.Render.
// It executes the specified template with the provided data and returns the result as a string.
func (r *GoTemplateRenderer) Render(templateName string, data interface{}) (string, error) {
	tmpl, exists := r.templates[templateName]
	if !exists {
		return "", domain.NewTemplateError(
			templateName,
			"template not found",
			nil,
		)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", domain.NewTemplateError(
			templateName,
			"failed to execute template",
			err,
		)
	}

	return buf.String(), nil
}

// GetLoadedTemplates returns the sorted names of all loaded templates.
// This is useful for debugging and validation.
func (r *GoTemplateRenderer) GetLoadedTemplates() []string {
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
