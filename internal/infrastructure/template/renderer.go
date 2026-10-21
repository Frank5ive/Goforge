package template

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"text/template"

	"github.com/Frank5ive/goforge/internal/domain"
)

// GoTemplateRenderer implements domain.TemplateRenderer using Go's text/template package.
type GoTemplateRenderer struct {
	templateDir string
	templates   map[string]*template.Template
	embeddedFS  *embed.FS // Optional embedded filesystem
}

// NewGoTemplateRenderer creates a new instance of GoTemplateRenderer.
// It loads and parses all templates from the specified directory.
func NewGoTemplateRenderer(templateDir string) (*GoTemplateRenderer, error) {
	renderer := &GoTemplateRenderer{
		templateDir: templateDir,
		templates:   make(map[string]*template.Template),
	}

	// Pre-load and parse all templates
	if err := renderer.loadTemplates(); err != nil {
		return nil, err
	}

	return renderer, nil
}

// NewGoTemplateRendererWithEmbed creates a renderer using embedded templates.
func NewGoTemplateRendererWithEmbed(embeddedFS embed.FS, subdirectory string) (*GoTemplateRenderer, error) {
	renderer := &GoTemplateRenderer{
		templates:  make(map[string]*template.Template),
		embeddedFS: &embeddedFS,
	}

	// Load templates from embedded filesystem
	if err := renderer.loadEmbeddedTemplates(subdirectory); err != nil {
		return nil, err
	}

	return renderer, nil
}

// Render implements domain.TemplateRenderer.Render.
// It executes the specified template with the provided data and returns the result as a string.
func (r *GoTemplateRenderer) Render(templateName string, data interface{}) (string, error) {
	// Check if template exists in cache
	tmpl, exists := r.templates[templateName]
	if !exists {
		return "", domain.NewTemplateError(
			templateName,
			"template not found in cache",
			nil,
		)
	}

	// Execute template
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

// loadTemplates loads and parses all .tmpl files from the template directory.
func (r *GoTemplateRenderer) loadTemplates() error {
	templateFiles := []string{
		"main.tmpl",
		"server.tmpl",
		"handler.tmpl",
		"gomod.tmpl",
		"logger.tmpl",
		"middleware.tmpl",
		"dockerfile.tmpl",
		"readme.tmpl",
		"gitignore.tmpl",
	}

	for _, fileName := range templateFiles {
		path := filepath.Join(r.templateDir, fileName)

		tmpl, err := template.ParseFiles(path)
		if err != nil {
			return domain.NewTemplateError(
				fileName,
				fmt.Sprintf("failed to parse template from %s", path),
				err,
			)
		}

		r.templates[fileName] = tmpl
	}

	return nil
}

// loadEmbeddedTemplates loads templates from an embedded filesystem.
func (r *GoTemplateRenderer) loadEmbeddedTemplates(subdirectory string) error {
	templateFiles := []string{
		"main.tmpl",
		"server.tmpl",
		"handler.tmpl",
		"gomod.tmpl",
		"logger.tmpl",
		"middleware.tmpl",
		"dockerfile.tmpl",
		"readme.tmpl",
		"gitignore.tmpl",
	}

	for _, fileName := range templateFiles {
		path := filepath.Join(subdirectory, fileName)

		// Read template content from embedded filesystem
		content, err := fs.ReadFile(*r.embeddedFS, path)
		if err != nil {
			return domain.NewTemplateError(
				fileName,
				fmt.Sprintf("failed to read embedded template %s", path),
				err,
			)
		}

		// Parse template content
		tmpl, err := template.New(fileName).Parse(string(content))
		if err != nil {
			return domain.NewTemplateError(
				fileName,
				fmt.Sprintf("failed to parse embedded template %s", fileName),
				err,
			)
		}

		r.templates[fileName] = tmpl
	}

	return nil
}

// GetLoadedTemplates returns the names of all loaded templates.
// This is useful for debugging and validation.
func (r *GoTemplateRenderer) GetLoadedTemplates() []string {
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}
