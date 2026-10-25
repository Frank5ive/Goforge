package domain

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Supported database engines.
const (
	DatabasePostgres = "postgres"
	DatabaseMySQL    = "mysql"
	DatabaseSQLite   = "sqlite"
	DatabaseMongo    = "mongo"
)

// DatabaseEngines lists the supported engines, for help text and errors.
var DatabaseEngines = []string{DatabasePostgres, DatabaseMySQL, DatabaseSQLite, DatabaseMongo}

// databaseAliases maps other common names to engine names.
var databaseAliases = map[string]string{
	"postgresql": DatabasePostgres,
	"pg":         DatabasePostgres,
	"mariadb":    DatabaseMySQL,
	"sqlite3":    DatabaseSQLite,
	"mongodb":    DatabaseMongo,
}

// databaseLabels names the engines for people.
var databaseLabels = map[string]string{
	DatabasePostgres: "PostgreSQL",
	DatabaseMySQL:    "MySQL",
	DatabaseSQLite:   "SQLite",
	DatabaseMongo:    "MongoDB",
}

// DatabaseLabel returns the display name of an engine, e.g. "PostgreSQL".
func DatabaseLabel(engine string) string {
	if label, ok := databaseLabels[engine]; ok {
		return label
	}
	return engine
}

// NormalizeDatabase returns the engine name for name, accepting aliases such as
// "postgresql" or "mariadb" in any case.
func NormalizeDatabase(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if engine, ok := databaseAliases[name]; ok {
		return engine
	}
	return name
}

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
	if !slices.Contains(DatabaseEngines, c.Engine) {
		return NewValidationErrorWithField("engine", fmt.Sprintf("unsupported database %q (supported: %s)", c.Engine, strings.Join(DatabaseEngines, ", ")))
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
	// Storage says where a new resource keeps its data; empty for other commands.
	Storage string
}

// DatabaseGenerator defines the contract for adding a database to an existing
// generated project.
type DatabaseGenerator interface {
	// Add connects the project to the database and moves every existing
	// resource onto it.
	// Cancelling ctx stops the dependency download; the project is left unchanged.
	Add(ctx context.Context, config *DatabaseConfig) (Changes, error)
}

// ModuleManager manages the Go module dependencies of a generated project.
type ModuleManager interface {
	// Require adds module at version to the project in dir and tidies its
	// go.mod and go.sum.
	// Cancelling ctx stops the go command.
	Require(ctx context.Context, dir, module, version string) error
}
