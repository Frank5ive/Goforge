package usecase

import (
	"fmt"
	"path/filepath"

	"github.com/Frank5ive/goforge/internal/domain"
)

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
// It orchestrates the entire server generation process.
func (u *ServerGeneratorUsecase) Generate(config *domain.ServerConfig) error {
	// Step 1: Validate configuration
	if err := config.Validate(); err != nil {
		return domain.NewGenerationError("validation", "configuration validation failed", err)
	}

	// Step 2: Check if project already exists
	projectPath := config.GetProjectPath()
	if u.fileWriter.FileExists(projectPath) {
		return domain.NewGenerationError(
			"directory_check",
			fmt.Sprintf("project directory already exists: %s", projectPath),
			nil,
		)
	}

	// Step 3: Create project directory structure
	if err := u.createProjectStructure(config); err != nil {
		return domain.NewGenerationError("create_structure", "failed to create project structure", err)
	}

	// Step 4: Generate main.go
	if err := u.generateMainFile(config); err != nil {
		return domain.NewGenerationError("generate_main", "failed to generate main.go", err)
	}

	// Step 5: Generate server.go
	if err := u.generateServerFile(config); err != nil {
		return domain.NewGenerationError("generate_server", "failed to generate server.go", err)
	}

	// Step 6: Generate health handler
	if err := u.generateHealthHandler(config); err != nil {
		return domain.NewGenerationError("generate_handler", "failed to generate health handler", err)
	}

	// Step 7: Generate go.mod
	if err := u.generateGoMod(config); err != nil {
		return domain.NewGenerationError("generate_gomod", "failed to generate go.mod", err)
	}

	// Step 8: Generate logger if requested
	if config.IncludeLogger {
		if err := u.generateLogger(config); err != nil {
			return domain.NewGenerationError("generate_logger", "failed to generate logger", err)
		}
	}

	// Step 9: Generate middleware
	if err := u.generateMiddleware(config); err != nil {
		return domain.NewGenerationError("generate_middleware", "failed to generate middleware", err)
	}

	// Step 10: Generate Dockerfile if requested
	if config.IncludeDocker {
		if err := u.generateDockerfile(config); err != nil {
			return domain.NewGenerationError("generate_dockerfile", "failed to generate Dockerfile", err)
		}
	}

	// Step 11: Generate README if requested
	if config.IncludeReadme {
		if err := u.generateReadme(config); err != nil {
			return domain.NewGenerationError("generate_readme", "failed to generate README", err)
		}
	}

	// Step 12: Generate .gitignore
	if err := u.generateGitignore(config); err != nil {
		return domain.NewGenerationError("generate_gitignore", "failed to generate .gitignore", err)
	}

	return nil
}

// createProjectStructure creates the directory structure for the project.
func (u *ServerGeneratorUsecase) createProjectStructure(config *domain.ServerConfig) error {
	projectPath := config.GetProjectPath()

	directories := []string{
		projectPath,
		filepath.Join(projectPath, "handlers"),
		filepath.Join(projectPath, "server"),
	}

	if config.IncludeLogger {
		directories = append(directories, filepath.Join(projectPath, "logger"))
	}

	for _, dir := range directories {
		if err := u.fileWriter.CreateDirectory(dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// generateMainFile generates the main.go file.
func (u *ServerGeneratorUsecase) generateMainFile(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("main.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "main.go")
	return u.fileWriter.WriteFile(path, content)
}

// generateServerFile generates the server.go file.
func (u *ServerGeneratorUsecase) generateServerFile(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("server.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "server", "server.go")
	return u.fileWriter.WriteFile(path, content)
}

// generateHealthHandler generates the health check handler.
func (u *ServerGeneratorUsecase) generateHealthHandler(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("handler.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "handlers", "health.go")
	return u.fileWriter.WriteFile(path, content)
}

// generateGoMod generates the go.mod file.
func (u *ServerGeneratorUsecase) generateGoMod(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("gomod.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "go.mod")
	return u.fileWriter.WriteFile(path, content)
}

// generateLogger generates the logger package.
func (u *ServerGeneratorUsecase) generateLogger(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("logger.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "logger", "logger.go")
	return u.fileWriter.WriteFile(path, content)
}

// generateMiddleware generates the middleware file.
func (u *ServerGeneratorUsecase) generateMiddleware(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("middleware.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "server", "middleware.go")
	return u.fileWriter.WriteFile(path, content)
}

// generateDockerfile generates the Dockerfile.
func (u *ServerGeneratorUsecase) generateDockerfile(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("dockerfile.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "Dockerfile")
	return u.fileWriter.WriteFile(path, content)
}

// generateReadme generates the README.md file.
func (u *ServerGeneratorUsecase) generateReadme(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("readme.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), "README.md")
	return u.fileWriter.WriteFile(path, content)
}

// generateGitignore generates the .gitignore file.
func (u *ServerGeneratorUsecase) generateGitignore(config *domain.ServerConfig) error {
	content, err := u.templateRenderer.Render("gitignore.tmpl", config)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetProjectPath(), ".gitignore")
	return u.fileWriter.WriteFile(path, content)
}
