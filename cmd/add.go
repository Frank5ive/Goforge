package cmd

import (
	"fmt"
	"io"
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
	cmd.AddCommand(newAddDBCmd())
	return cmd
}

// addResourceOptions holds the flag values for the add resource command.
type addResourceOptions struct {
	fields string
	plural string
	dir    string
	dryRun bool
	json   bool
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
ID and timestamps are added automatically.

Fields are name:type, separated by commas. Types: %s.
  - string and email fields are required; a trailing ? makes one optional:
    image_url:string?
  - email fields must be email addresses: contact:email
  - a (min..max) range bounds a number, or a string's length; either end may be
    left out: name:string(1..100), price:float(0..), quantity:int(1..)
  - ref(<resource>) is the ID of an item of another resource (or this one), which
    must exist (else 422) and can't be deleted while referenced (409):
    category_id:ref(category), manager_id:ref(employee)?
Each rule is checked by the generated Validate (with a test per rule), and the
resource's rules.go is the place for any other business rules.`, fieldTypeNames()),
		Example: `  goforge add resource product --fields "name:string(1..100),price:float(0..),in_stock:bool"
  goforge add resource customer --fields "name:string,email:email,phone:string?"
  goforge add resource blog_post --fields "title:string,body:string,cover_url:string?,published_at:time"
  goforge add resource person --plural people --fields "name:string,age:int"
  goforge add resource task --dry-run
  goforge add resource order --fields "total:float,paid:bool" --json`,
		Args: usageArgs(cobra.ExactArgs(1)),
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
	flags.BoolVar(&opts.json, "json", false, "Print the result as JSON, for scripts and AI agents")

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

	changes, err := container.ResourceGeneratorUsecase.Add(config)
	if err != nil {
		return err
	}

	r := newResult("add resource", opts.dir, opts.dryRun)
	r.Created, r.Updated = changes.Created, changes.Updated
	if opts.dryRun {
		r.Summary = fmt.Sprintf("GoForge would add the %s resource.", config.Name)
	} else {
		r.Summary = fmt.Sprintf("Added the %s resource", config.Name)
	}

	r.Endpoints = resourceEndpoints(config.RoutePath())

	if !opts.dryRun {
		pkg := config.PackageName()
		r.Info = append(r.Info,
			"The routes are registered in server/routes.go, so they work as soon as the server starts (or restarts).",
			fmt.Sprintf("Try them with %s/requests.http (VS Code REST Client or JetBrains HTTP client).", pkg))
		if changes.Storage != "" {
			r.Info = append(r.Info, changes.Storage)
		}
		r.NextSteps = []nextStep{
			{Command: "go test ./" + pkg, Description: "run the resource's tests"},
			{Command: "make run", Description: "start the server in development mode"},
		}
	}

	return r.print(out, opts.json)
}

func fieldTypeNames() string {
	names := make([]string, len(domain.FieldTypes))
	for i, t := range domain.FieldTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

// resourceEndpoints lists the routes a resource at path serves.
func resourceEndpoints(path string) []endpoint {
	return []endpoint{
		{Method: "GET", Path: path, Query: "limit=20&offset=0"},
		{Method: "POST", Path: path},
		{Method: "GET", Path: path + "/{id}"},
		{Method: "PUT", Path: path + "/{id}"},
		{Method: "PATCH", Path: path + "/{id}"},
		{Method: "DELETE", Path: path + "/{id}"},
	}
}
