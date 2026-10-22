package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add code to a project created by goforge generate",
	}
	cmd.AddCommand(newAddResourceCmd())
	return cmd
}

// addResourceOptions holds the flag values for the add resource command.
type addResourceOptions struct {
	fields string
	plural string
	dir    string
	dryRun bool
}

func newAddResourceCmd() *cobra.Command {
	opts := &addResourceOptions{}

	cmd := &cobra.Command{
		Use:   "resource <name>",
		Short: "Add a REST resource with CRUD endpoints, storage, and tests",
		Long: fmt.Sprintf(`Add a REST resource to a project created by goforge generate.

The resource gets its own package with a model and validation rules, a storage
interface with an in-memory implementation, a service for business logic, HTTP
handlers for list (paginated), create, get, replace (PUT), partial update (PATCH),
and delete, tests, and a requests.http file of example requests. Its routes are
registered in server/routes.go automatically, so they work as soon as the server
starts.

<name> is the singular resource name in snake_case, e.g. product or blog_post.
Field types: %s. String fields are required unless marked
optional with a trailing "?", e.g. "image_url:string?". ID and timestamps are
added automatically.`, fieldTypeNames()),
		Example: `  goforge add resource product --fields "name:string,price:float,in_stock:bool"
  goforge add resource blog_post --fields "title:string,body:string,cover_url:string?,published_at:time"
  goforge add resource person --plural people --fields "name:string,age:int"
  goforge add resource task --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runAddResource(cmd.OutOrStdout(), args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.fields, "fields", "f", "name:string", `Comma-separated fields as name:type, e.g. "title:string,price:float"`)
	flags.StringVar(&opts.plural, "plural", "", "Plural name for irregular words, e.g. people (default: derived from <name>)")
	flags.StringVarP(&opts.dir, "dir", "d", ".", "Project directory (the one containing go.mod)")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Show the files that would be created or changed without writing anything")

	return cmd
}

func runAddResource(out io.Writer, name string, opts *addResourceOptions) error {
	fields, err := domain.ParseFields(opts.fields)
	if err != nil {
		return err
	}

	config := &domain.ResourceConfig{
		ProjectDir: opts.dir,
		Name:       strings.ReplaceAll(name, "-", "_"),
		Plural:     strings.ReplaceAll(opts.plural, "-", "_"),
		Fields:     fields,
	}

	container, err := di.BuildContainer(templatesFS, di.Options{DryRun: opts.dryRun})
	if err != nil {
		return err
	}

	if err := container.ResourceGeneratorUsecase.Add(config); err != nil {
		return err
	}

	pkgDir := filepath.Join(config.ProjectDir, config.PackageName())
	var created, updated []string
	for _, file := range container.Files.Files() {
		if rel, err := filepath.Rel(pkgDir, file); err == nil && !strings.HasPrefix(rel, "..") {
			created = append(created, file)
		} else {
			updated = append(updated, file)
		}
	}

	if opts.dryRun {
		fmt.Fprintf(out, "🔍 Dry run: no files were written. GoForge would create %d files:\n", len(created))
	} else {
		fmt.Fprintf(out, "✅ Added the %s resource. Created %d files:\n", config.Name, len(created))
	}
	printFiles(out, config.ProjectDir, created)

	if len(updated) > 0 {
		if opts.dryRun {
			fmt.Fprintln(out, "\n…and would update:")
		} else {
			fmt.Fprintln(out, "\n📝 Updated:")
		}
		printFiles(out, config.ProjectDir, updated)
	}

	path := config.RoutePath()
	fmt.Fprintf(out, "\n🌐 Endpoints:\n")
	fmt.Fprintf(out, "   GET    %s?limit=20&offset=0\n", path)
	fmt.Fprintf(out, "   POST   %s\n", path)
	fmt.Fprintf(out, "   GET    %s/{id}\n", path)
	fmt.Fprintf(out, "   PUT    %s/{id}\n", path)
	fmt.Fprintf(out, "   PATCH  %s/{id}\n", path)
	fmt.Fprintf(out, "   DELETE %s/{id}\n", path)

	if !opts.dryRun {
		pkg := config.PackageName()
		fmt.Fprintf(out, "\n🚀 The routes are registered in server/routes.go, so they're live as soon as\n")
		fmt.Fprintf(out, "   you start (or restart) the server: go run .\n")
		fmt.Fprintf(out, "📮 Try them with %s/requests.http (VS Code REST Client or JetBrains HTTP client)\n", pkg)
		fmt.Fprintf(out, "🧪 Run its tests with: go test ./%s\n", pkg)
		fmt.Fprintf(out, "💾 Data is stored in memory. Implement %s.Repository to use a database.\n", pkg)
	}

	return nil
}

func fieldTypeNames() string {
	names := make([]string, len(domain.FieldTypes))
	for i, t := range domain.FieldTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
