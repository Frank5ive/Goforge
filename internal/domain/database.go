package domain

import (
	"fmt"
	"strings"
)

// DatabasePostgres is the only supported database engine.
const DatabasePostgres = "postgres"

// DatabaseConfig describes a database to add to an existing GoForge project.
type DatabaseConfig struct {
	// ProjectDir is the root of the project (the directory containing go.mod).
	ProjectDir string
	// Engine is the database engine, e.g. "postgres".
	Engine string
}

// Validate checks the project directory and engine.
func (c *DatabaseConfig) Validate() error {
	if strings.TrimSpace(c.ProjectDir) == "" {
		return NewValidationErrorWithField("dir", "project directory cannot be empty")
	}
	if c.Engine != DatabasePostgres {
		return NewValidationErrorWithField("engine", fmt.Sprintf("unsupported database %q (supported: %s)", c.Engine, DatabasePostgres))
	}
	return nil
}

// Changes lists the files a command created and updated, relative to the
// project directory.
type Changes struct {
	Created []string
	Updated []string
	// Notes are follow-up steps the command could not do itself.
	Notes []string
}

// DatabaseGenerator defines the contract for adding a database to an existing
// generated project.
type DatabaseGenerator interface {
	// Add connects the project to the database and moves every existing
	// resource onto it.
	Add(config *DatabaseConfig) (Changes, error)
}

// ModuleManager manages the Go module dependencies of a generated project.
type ModuleManager interface {
	// Require adds module at version to the project in dir and tidies its
	// go.mod and go.sum.
	Require(dir, module, version string) error
}
