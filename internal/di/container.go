package di

import (
	"embed"
	"log"

	"github.com/Frank5ive/goforge/internal/domain"
	"github.com/Frank5ive/goforge/internal/infrastructure/filesystem"
	"github.com/Frank5ive/goforge/internal/infrastructure/template"
	"github.com/Frank5ive/goforge/internal/usecase"
)

// Container holds the application's dependencies.
type Container struct {
	ServerGeneratorUsecase domain.ServerGenerator
}

// BuildContainer creates and wires the application's dependencies.
func BuildContainer(templateDir string) *Container {
	// Instantiate infrastructure components
	fileWriter := filesystem.NewFileSystemWriter()
	templateRenderer, err := template.NewGoTemplateRenderer(templateDir)
	if err != nil {
		log.Fatalf("failed to create template renderer: %v", err)
	}

	// Instantiate usecase and inject dependencies
	serverGeneratorUsecase := usecase.NewServerGeneratorUsecase(templateRenderer, fileWriter)

	return &Container{
		ServerGeneratorUsecase: serverGeneratorUsecase,
	}
}

// BuildContainerWithEmbed creates and wires dependencies using embedded templates.
func BuildContainerWithEmbed(embeddedFS embed.FS, subdirectory string) *Container {
	// Instantiate infrastructure components
	fileWriter := filesystem.NewFileSystemWriter()
	templateRenderer, err := template.NewGoTemplateRendererWithEmbed(embeddedFS, subdirectory)
	if err != nil {
		log.Fatalf("failed to create template renderer with embedded templates: %v", err)
	}

	// Instantiate usecase and inject dependencies
	serverGeneratorUsecase := usecase.NewServerGeneratorUsecase(templateRenderer, fileWriter)

	return &Container{
		ServerGeneratorUsecase: serverGeneratorUsecase,
	}
}
