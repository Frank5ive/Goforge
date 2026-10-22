package di

import (
	"fmt"
	"io/fs"

	"github.com/Frank5ive/goforge/internal/domain"
	"github.com/Frank5ive/goforge/internal/infrastructure/filesystem"
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
	ServerGeneratorUsecase domain.ServerGenerator

	// Files records every file written by ServerGeneratorUsecase
	// (or that would be written, in dry-run mode).
	Files *filesystem.RecordingWriter
}

// BuildContainer creates and wires the application's dependencies.
// templates must contain the *.tmpl files at its root, e.g. os.DirFS(dir)
// or fs.Sub(embedded, "templates").
func BuildContainer(templates fs.FS, opts Options) (*Container, error) {
	templateRenderer, err := template.NewGoTemplateRenderer(templates)
	if err != nil {
		return nil, fmt.Errorf("failed to create template renderer: %w", err)
	}

	files := filesystem.NewRecordingWriter(filesystem.NewFileSystemWriter(), opts.DryRun)

	return &Container{
		ServerGeneratorUsecase: usecase.NewServerGeneratorUsecase(templateRenderer, files),
		Files:                  files,
	}, nil
}
