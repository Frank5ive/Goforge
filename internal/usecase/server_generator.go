package usecase

import (
	"fmt"
	"go/format"
	"path/filepath"

	"github.com/Frank5ive/goforge/internal/domain"
)

// projectFile describes one file in a generated project.
type projectFile struct {
	// template is the name of the template that renders the file.
	template string
	// path is the file's location relative to the project root, slash-separated.
	path string
	// include reports whether the file is generated for a configuration.
	include func(config *domain.ServerConfig) bool
}

func always(*domain.ServerConfig) bool       { return true }
func withLogger(c *domain.ServerConfig) bool { return c.IncludeLogger }
func withDocker(c *domain.ServerConfig) bool { return c.IncludeDocker }
func withReadme(c *domain.ServerConfig) bool { return c.IncludeReadme }
func withCI(c *domain.ServerConfig) bool     { return c.IncludeCI }

// projectFiles lists every file GoForge can generate, in generation order.
// Adding a file to generated projects means adding a template and a line here.
var projectFiles = []projectFile{
	{template: "main.tmpl", path: "main.go", include: always},
	{template: "gomod.tmpl", path: "go.mod", include: always},
	{template: "config.tmpl", path: "config/config.go", include: always},
	{template: "config_test.tmpl", path: "config/config_test.go", include: always},
	{template: "server.tmpl", path: "server/server.go", include: always},
	{template: "routes.tmpl", path: "server/routes.go", include: always},
	{template: "middleware.tmpl", path: "server/middleware.go", include: always},
	{template: "server_test.tmpl", path: "server/server_test.go", include: always},
	{template: "handler.tmpl", path: "handlers/health.go", include: always},
	{template: "respond.tmpl", path: "handlers/respond.go", include: always},
	{template: "respond_test.tmpl", path: "handlers/respond_test.go", include: always},
	{template: "health_test.tmpl", path: "handlers/health_test.go", include: always},
	{template: "logger.tmpl", path: "logger/logger.go", include: withLogger},
	{template: "logger_test.tmpl", path: "logger/logger_test.go", include: withLogger},
	{template: "makefile.tmpl", path: "Makefile", include: always},
	{template: "golangci.tmpl", path: ".golangci.yml", include: always},
	{template: "ci.tmpl", path: ".github/workflows/ci.yml", include: withCI},
	{template: "dockerfile.tmpl", path: "Dockerfile", include: withDocker},
	{template: "dockerignore.tmpl", path: ".dockerignore", include: withDocker},
	{template: "readme.tmpl", path: "README.md", include: withReadme},
	{template: "gitignore.tmpl", path: ".gitignore", include: always},
}

// ServerGeneratorUsecase orchestrates the server generation process.
// It implements the domain.ServerGenerator interface.
type ServerGeneratorUsecase struct {
	templateRenderer domain.TemplateRenderer
	fileWriter       domain.FileWriter
}

// NewServerGeneratorUsecase creates a new instance of ServerGeneratorUsecase.
// It uses constructor injection to receive its dependencies.
func NewServerGeneratorUsecase(
	renderer domain.TemplateRenderer,
	writer domain.FileWriter,
) *ServerGeneratorUsecase {
	return &ServerGeneratorUsecase{
		templateRenderer: renderer,
		fileWriter:       writer,
	}
}

// Generate implements the domain.ServerGenerator interface.
// It validates the configuration, then renders and writes every project file.
// If any file fails, the partially generated project directory is removed.
func (u *ServerGeneratorUsecase) Generate(config *domain.ServerConfig) error {
	if err := config.Validate(); err != nil {
		return domain.NewGenerationError("validation", "configuration validation failed", err)
	}

	projectPath := config.GetProjectPath()
	if u.fileWriter.FileExists(projectPath) {
		return domain.NewGenerationError(
			"directory_check",
			fmt.Sprintf("project directory already exists: %s", projectPath),
			nil,
		)
	}

	if err := u.fileWriter.CreateDirectory(projectPath); err != nil {
		return domain.NewGenerationError("create_structure", "failed to create project directory", err)
	}

	for _, file := range projectFiles {
		if !file.include(config) {
			continue
		}

		if err := u.generateFile(config, file); err != nil {
			genErr := domain.NewGenerationError("generate_files", fmt.Sprintf("failed to generate %s", file.path), err)
			if cleanupErr := u.fileWriter.RemoveAll(projectPath); cleanupErr != nil {
				genErr.Message += fmt.Sprintf(" (cleanup of %s also failed: %v)", projectPath, cleanupErr)
			}
			return genErr
		}
	}

	return nil
}

// generateFile renders a single project file and writes it under the project root.
func (u *ServerGeneratorUsecase) generateFile(config *domain.ServerConfig, file projectFile) error {
	content, err := u.templateRenderer.Render(file.template, config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), filepath.FromSlash(file.path))
	return u.writeFile(path, content)
}

// writeFile writes generated content to path. Go source is run through gofmt first,
// so output is formatted regardless of template whitespace and invalid Go is caught here.
func (u *ServerGeneratorUsecase) writeFile(path string, content string) error {
	if filepath.Ext(path) == ".go" {
		formatted, err := format.Source([]byte(content))
		if err != nil {
			return fmt.Errorf("generated %s is not valid Go: %w", filepath.Base(path), err)
		}
		content = string(formatted)
	}

	return u.fileWriter.WriteFile(path, content)
}
