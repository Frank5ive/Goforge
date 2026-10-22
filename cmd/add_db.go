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
		Use:   "db postgres",
		Short: "Store data in PostgreSQL instead of memory",
		Long: `Add PostgreSQL to a project created by goforge generate.

This adds a db package (connection pool and embedded SQL migrations), connects it
at startup, makes /readyz check the database, and moves every existing resource
onto PostgreSQL: each gets a PostgreSQL repository, tests, and a migration that
creates its table. Resources added afterwards use PostgreSQL automatically.

It also adds docker-compose.yml with a local PostgreSQL, Makefile targets
(db-up, db-down, db-reset, test-db), a database for tests in CI, and the pgx
driver to go.mod. If any step fails, the project is left unchanged.`,
		Example: `  goforge add db postgres
  goforge add db postgres --dry-run`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{domain.DatabasePostgres},
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

	changes, err := container.DatabaseGeneratorUsecase.Add(&domain.DatabaseConfig{ProjectDir: opts.dir, Engine: engine})
	if err != nil {
		return err
	}

	if opts.dryRun {
		fmt.Fprintf(out, "🔍 Dry run: no files were written. GoForge would create %d files:\n", len(changes.Created))
	} else {
		fmt.Fprintf(out, "✅ Added PostgreSQL. Created %d files:\n", len(changes.Created))
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

	fmt.Fprintln(out, "\n🐘 Next steps:")
	fmt.Fprintln(out, "   make db-up     # start PostgreSQL in Docker")
	fmt.Fprintln(out, "   go run .       # connects, applies migrations, and serves your resources from PostgreSQL")
	fmt.Fprintln(out, "   make test-db   # run every test, including the PostgreSQL ones")
	fmt.Fprintln(out, "\n🔐 DATABASE_URL defaults to the local Docker database; set it for production.")
	return nil
}

// printRelative lists paths that are already relative to the project.
func printRelative(out io.Writer, paths []string) {
	for _, path := range paths {
		fmt.Fprintf(out, "   %s\n", path)
	}
}
