package cmd

import (
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
  goforge add db mysql --dry-run`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: domain.DatabaseEngines,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runAddDB(cmd.OutOrStdout(), args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.dir, "dir", "d", ".", "Project directory (the one containing go.mod)")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Show the files that would be created or changed without writing anything")

	return cmd
}

func runAddDB(out io.Writer, engine string, opts *addDBOptions) error {
	container, err := di.BuildContainer(templatesFS, di.Options{DryRun: opts.dryRun})
	if err != nil {
		return err
	}

	engine = domain.NormalizeDatabase(engine)
	label := domain.DatabaseLabel(engine)

	changes, err := container.DatabaseGeneratorUsecase.Add(&domain.DatabaseConfig{ProjectDir: opts.dir, Engine: engine})
	if err != nil {
		return err
	}

	if opts.dryRun {
		fmt.Fprintf(out, "🔍 Dry run: no files were written. GoForge would create %d files:\n", len(changes.Created))
	} else {
		fmt.Fprintf(out, "✅ Added %s. Created %d files:\n", label, len(changes.Created))
	}
	printRelative(out, changes.Created)

	if opts.dryRun {
		fmt.Fprintln(out, "\n…and would update:")
	} else {
		fmt.Fprintln(out, "\n📝 Updated:")
	}
	printRelative(out, changes.Updated)

	for _, note := range changes.Notes {
		fmt.Fprintf(out, "\n⚠️  %s\n", note)
	}

	if opts.dryRun {
		return nil
	}

	fmt.Fprintln(out, "\n🗄️  Next steps:")
	if engine == domain.DatabaseSQLite {
		fmt.Fprintln(out, "   go run .       # creates the database file, applies migrations, and serves your resources")
		fmt.Fprintln(out, "   go test ./...  # the SQLite tests run against a temporary database file")
		fmt.Fprintln(out, "\n🔐 DATABASE_URL defaults to a file in the project directory; set it for production.")
		return nil
	}
	fmt.Fprintf(out, "   make db-up     # start %s in Docker\n", label)
	fmt.Fprintf(out, "   go run .       # connects and serves your resources from %s\n", label)
	fmt.Fprintf(out, "   make test-db   # run every test, including the %s ones\n", label)
	fmt.Fprintln(out, "\n🔐 DATABASE_URL defaults to the local Docker database; set it for production.")
	return nil
}

// printRelative lists paths that are already relative to the project.
func printRelative(out io.Writer, paths []string) {
	for _, path := range paths {
		fmt.Fprintf(out, "   %s\n", path)
	}
}
