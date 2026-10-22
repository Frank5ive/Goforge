package di

import (
	"fmt"
	"io/fs"

	"github.com/Frank5ive/goforge/internal/domain"
	"github.com/Frank5ive/goforge/internal/infrastructure/filesystem"
	"github.com/Frank5ive/goforge/internal/infrastructure/gomod"
	"github.com/Frank5ive/goforge/internal/infrastructure/template"
	"github.com/Frank5ive/goforge/internal/usecase"
)

// Options configures how the container is built.
type Options struct {
	// DryRun makes generation record the files it would write without writing them.
	DryRun bool
}

// Container holds the application's dependencies.
type Container struct {
	ServerGeneratorUsecase   domain.ServerGenerator
	ResourceGeneratorUsecase domain.ResourceGenerator
	DatabaseGeneratorUsecase domain.DatabaseGenerator

	// Files records every file written by ServerGeneratorUsecase
	// (or that would be written, in dry-run mode).
	Files *filesystem.RecordingWriter
}

// BuildContainer creates and wires the application's dependencies.
// templates is the templates directory, e.g. os.DirFS(dir) or
// fs.Sub(embedded, "templates"); *.tmpl files are loaded from it recursively.
func BuildContainer(templates fs.FS, opts Options) (*Container, error) {
	templateRenderer, err := template.NewGoTemplateRenderer(templates)
	if err != nil {
		return nil, fmt.Errorf("failed to create template renderer: %w", err)
	}

	files := filesystem.NewRecordingWriter(filesystem.NewFileSystemWriter(), opts.DryRun)

	return &Container{
		ServerGeneratorUsecase:   usecase.NewServerGeneratorUsecase(templateRenderer, files),
		ResourceGeneratorUsecase: usecase.NewResourceGeneratorUsecase(templateRenderer, files),
		DatabaseGeneratorUsecase: usecase.NewDatabaseGeneratorUsecase(templateRenderer, files, gomod.New(opts.DryRun)),
		Files:                    files,
	}, nil
}
