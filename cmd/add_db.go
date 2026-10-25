package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

// addDBOptions holds the flag values for the add db command.
type addDBOptions struct {
	dir    string
	dryRun bool
	json   bool
}

func newAddDBCmd() *cobra.Command {
	opts := &addDBOptions{}

	cmd := &cobra.Command{
		Use:   "db <postgres|mysql|sqlite|mongo>",
		Short: "Store data in a database instead of memory",
		Long: `Add a database to a project created by goforge generate.

Supported: postgres (PostgreSQL), mysql (MySQL or MariaDB), sqlite (SQLite, no
server needed), and mongo (MongoDB).

This adds a db package (connection and, for SQL databases, embedded migrations),
connects it at startup, makes /readyz check the database, and moves every
existing resource onto it: each gets a repository for the database and tests,
plus a migration that creates its table for SQL databases. Resources added
afterwards use the database automatically.

It also adds local tooling (docker-compose.yml and Makefile targets, except for
SQLite, which is just a file), a database for tests in CI, and the driver to
go.mod. If any step fails, the project is left unchanged.`,
		Example: `  goforge add db postgres
  goforge add db sqlite
  goforge add db mysql --dry-run
  goforge add db mongo --json`,
		Args:      usageArgs(cobra.ExactArgs(1)),
		ValidArgs: domain.DatabaseEngines,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runAddDB(commandContext(cmd), cmd.OutOrStdout(), args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.dir, "dir", "d", ".", "Project directory (the one containing go.mod)")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Show the files that would be created or changed without writing anything")
	flags.BoolVar(&opts.json, "json", false, "Print the result as JSON, for scripts and AI agents")

	return cmd
}

func runAddDB(ctx context.Context, out io.Writer, engine string, opts *addDBOptions) error {
	container, err := di.BuildContainer(templatesFS, di.Options{DryRun: opts.dryRun})
	if err != nil {
		return err
	}

	engine = domain.NormalizeDatabase(engine)
	label := domain.DatabaseLabel(engine)

	changes, err := container.DatabaseGeneratorUsecase.Add(ctx, &domain.DatabaseConfig{ProjectDir: opts.dir, Engine: engine})
	if err != nil {
		return err
	}

	r := newResult("add db", opts.dir, opts.dryRun)
	r.Created, r.Updated, r.Notes = changes.Created, changes.Updated, changes.Notes
	if opts.dryRun {
		r.Summary = fmt.Sprintf("GoForge would add %s.", label)
		return r.print(out, opts.json)
	}

	r.Summary = "Added " + label
	if engine == domain.DatabaseSQLite {
		r.NextSteps = []nextStep{
			{Command: "make run", Description: "creates the database file, applies migrations, and serves your resources"},
			{Command: "go test ./...", Description: "the SQLite tests run against a temporary database file"},
		}
		r.Info = append(r.Info, "DATABASE_URL defaults to a file in the project directory; set it for production.")
		return r.print(out, opts.json)
	}
	r.NextSteps = []nextStep{
		{Command: "make db-up", Description: "start " + label + " in Docker"},
		{Command: "make run", Description: "connects and serves your resources from " + label},
		{Command: "make test-db", Description: "run every test, including the " + label + " ones"},
	}
	r.Info = append(r.Info, "DATABASE_URL defaults to the local Docker database; set it for production.")
	return r.print(out, opts.json)
}
