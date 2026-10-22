package usecase

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Frank5ive/goforge/internal/domain"
)

// routesMarker marks where `goforge add resource` registers new resources in a
// generated project's server/routes.go.
const routesMarker = "// goforge:routes"

// resourceFiles lists the files generated for a resource, in generation order.
// file returns the name within the resource package directory.
var resourceFiles = []struct {
	template string
	file     func(config *domain.ResourceConfig) string
}{
	{"resource/model.tmpl", func(c *domain.ResourceConfig) string { return c.Name + ".go" }},
	{"resource/memory.tmpl", func(*domain.ResourceConfig) string { return "memory.go" }},
	{"resource/service.tmpl", func(*domain.ResourceConfig) string { return "service.go" }},
	{"resource/handler.tmpl", func(*domain.ResourceConfig) string { return "handler.go" }},
	{"resource/service_test.tmpl", func(*domain.ResourceConfig) string { return "service_test.go" }},
	{"resource/handler_test.tmpl", func(*domain.ResourceConfig) string { return "handler_test.go" }},
	{"resource/repository_test.tmpl", func(*domain.ResourceConfig) string { return "repository_test.go" }},
	{"resource/requests.tmpl", func(*domain.ResourceConfig) string { return "requests.http" }},
}

// defaultPortPattern finds the default port in a generated project's config/config.go.
var defaultPortPattern = regexp.MustCompile(`defaultPort\s*=\s*(\d+)`)

// fallbackPort is used for requests.http when the project's port can't be read.
const fallbackPort = 8080

// ResourceGeneratorUsecase adds REST resources to existing generated projects.
// It implements the domain.ResourceGenerator interface.
type ResourceGeneratorUsecase struct {
	templateRenderer domain.TemplateRenderer
	fileWriter       domain.FileWriter
	now              func() time.Time // dates migrations
}

// NewResourceGeneratorUsecase creates a new instance of ResourceGeneratorUsecase.
func NewResourceGeneratorUsecase(
	renderer domain.TemplateRenderer,
	writer domain.FileWriter,
) *ResourceGeneratorUsecase {
	return &ResourceGeneratorUsecase{
		templateRenderer: renderer,
		fileWriter:       writer,
		now:              time.Now,
	}
}

// Add implements domain.ResourceGenerator. Everything is rendered and checked
// before any file is written, then applied as one changeset, so a failed run
// leaves the project unchanged. In a project with a database, the resource is
// stored in it and, for SQL databases, gets a migration for its table.
func (u *ResourceGeneratorUsecase) Add(config *domain.ResourceConfig) (domain.Changes, error) {
	if err := config.Validate(); err != nil {
		return domain.Changes{}, domain.NewGenerationError("validation", "resource validation failed", err)
	}
	dir := config.ProjectDir

	modulePath, err := readModulePath(u.fileWriter, dir)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("project_check",
			fmt.Sprintf("%s is not a Go module; run this inside a project created by goforge generate", dir), err)
	}

	manifest, hasManifest, err := readManifest(u.fileWriter, dir)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("project_check", "cannot read "+domain.ManifestFile, err)
	}
	if !hasManifest {
		// Projects from older GoForge versions have no manifest; start one
		manifest = domain.Manifest{Module: modulePath}
	}
	var eng *engine
	if manifest.Database != "" {
		if eng, err = engineFor(manifest.Database); err != nil {
			return domain.Changes{}, domain.NewGenerationError("project_check", "cannot read "+domain.ManifestFile, err)
		}
	}

	routesPath := filepath.Join(dir, "server", "routes.go")
	routes, err := u.fileWriter.ReadFile(routesPath)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("project_check",
			"server/routes.go not found; run this inside a project created by goforge generate", err)
	}

	pkg := config.PackageName()
	repo := pkg + ".NewMemoryRepository()"
	if eng != nil {
		repo = fmt.Sprintf("%s.New%s(deps.DB)", pkg, eng.repoType)
	}
	updatedRoutes, err := registerResource(routes, modulePath, pkg, repo)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("register_routes", "cannot register the resource in server/routes.go", err)
	}

	pkgDir := filepath.Join(dir, pkg)
	if u.fileWriter.FileExists(pkgDir) {
		return domain.Changes{}, domain.NewGenerationError("directory_check", fmt.Sprintf("%s already exists", pkgDir), nil)
	}

	data := newResourceData(config, modulePath)
	data.Port = u.readDefaultPort(dir)

	cs := newChangeset(u.fileWriter)
	for _, f := range resourceFiles {
		if err := planTemplate(u.templateRenderer, cs, filepath.Join(pkgDir, f.file(config)), f.template, data); err != nil {
			return domain.Changes{}, domain.NewGenerationError("generate_files", fmt.Sprintf("failed to generate %s", f.file(config)), err)
		}
	}
	if eng != nil {
		if err := planDatabaseResource(u.templateRenderer, cs, dir, eng, config, data, u.now().UTC()); err != nil {
			return domain.Changes{}, domain.NewGenerationError("generate_files", "failed to generate the "+eng.label+" repository", err)
		}
	}
	cs.edit(routesPath, updatedRoutes)

	manifest.AddResource(config)
	if err := planManifest(cs, dir, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("manifest", "cannot update "+domain.ManifestFile, err)
	}

	if err := cs.apply(nil, nil); err != nil {
		return domain.Changes{}, domain.NewGenerationError("apply", "the project was left unchanged", err)
	}
	return domain.Changes{Created: cs.createdIn(dir), Updated: cs.editedIn(dir), Storage: storage(eng)}, nil
}

// readDefaultPort returns the default port from the project's config/config.go, or
// fallbackPort for projects that don't have one (e.g. older GoForge versions).
func (u *ResourceGeneratorUsecase) readDefaultPort(projectDir string) int {
	src, err := u.fileWriter.ReadFile(filepath.Join(projectDir, "config", "config.go"))
	if err != nil {
		return fallbackPort
	}

	match := defaultPortPattern.FindStringSubmatch(src)
	if match == nil {
		return fallbackPort
	}
	port, err := strconv.Atoi(match[1])
	if err != nil {
		return fallbackPort
	}
	return port
}

// registerResource returns routes.go source with the resource package imported
// and its routes registered just above the goforge:routes marker. repo is the Go
// expression that creates its Repository, e.g. "products.NewMemoryRepository()".
func registerResource(src, modulePath, pkg, repo string) (string, error) {
	importPath := modulePath + "/" + pkg
	registration := fmt.Sprintf("%s.RegisterRoutes(mux, %s.NewService(%s))", pkg, pkg, repo)

	if strings.Contains(src, strconv.Quote(importPath)) {
		return "", fmt.Errorf("%q is already imported", importPath)
	}

	src, err := insertAboveMarker(src, routesMarker, registration)
	if err != nil {
		return "", fmt.Errorf("%w; add it inside registerRoutes, or register the resource yourself with:\n\t%s", err, registration)
	}
	if src, err = addImport(src, importPath); err != nil {
		return "", err
	}
	return formatGo("routes.go", src)
}

// resourceData is the template data for resource templates.
type resourceData struct {
	Module      string // module path of the project, e.g. "github.com/acme/api"
	Package     string // Go package, e.g. "blogposts"
	Type        string // Go type, e.g. "BlogPost"
	Human       string // e.g. "blog post"
	HumanPlural string // e.g. "blog posts"
	Path        string // collection route, e.g. "/blog-posts"
	Fields      []resourceField
	First       resourceField // the first field, used by PATCH examples and tests
	// RequiredField is the Go name of the first required string field, or "" if
	// there is none (then Validate has no rules).
	RequiredField string
	Port          int    // the project's default port, for requests.http
	SampleBody    string // pretty-printed JSON create request, for requests.http
	UpdatedBody   string // pretty-printed JSON replace request, for requests.http

	// Database-specific names, set by addEngineData for projects with a database
	Engine      string // e.g. "postgres"
	RepoType    string // e.g. "PostgresRepository"
	EngineLabel string // e.g. "PostgreSQL"
	TableName   string // unquoted table or collection name, e.g. blog_posts

	// SQL names and statements; identifiers are quoted so names like "order"
	// or "user" work
	Table         string // quoted table name
	IndexName     string
	IDColumn      string
	CreatedColumn string
	UpdatedColumn string
	IDType        string
	TimeType      string
	Columns       string // the select list, in scan order
	InsertQuery   string
	UpdateQuery   string
	TextTimes     bool // times are stored as text (SQLite)

	// The statements as Go string literals, for templates that can't use raw
	// strings (MySQL quotes identifiers with backticks)
	CountGo, ListGo, GetGo, InsertGo, UpdateGo, DeleteGo, ClearGo string
	CreatedArg, UpdatedArg                                        string // Go expressions for the timestamp arguments
}

// resourceField describes one field for the templates, including sample values
// used by the generated tests.
type resourceField struct {
	GoName      string
	JSON        string
	GoType      string
	Type        domain.FieldType
	Column      string // quoted SQL column name, set by addEngineData
	SQLType     string // set by addEngineData
	ArgExpr     string // Go expression passing the field to SQL, set by addEngineData
	TextVar     string // name of the variable a text-stored time is scanned into
	IsTime      bool
	Required    bool   // a required string field
	Sample      string // Go literal
	Updated     string // a different Go literal
	SampleJSON  string // Sample as JSON
	UpdatedJSON string // Updated as JSON
}

func newResourceData(config *domain.ResourceConfig, modulePath string) resourceData {
	data := resourceData{
		Module:      modulePath,
		Package:     config.PackageName(),
		Type:        domain.CamelCase(config.Name),
		Human:       strings.ReplaceAll(config.Name, "_", " "),
		HumanPlural: strings.ReplaceAll(config.PluralName(), "_", " "),
		Path:        config.RoutePath(),
	}

	for _, f := range config.Fields {
		field := resourceField{
			GoName:   domain.CamelCase(f.Name),
			JSON:     f.Name,
			Type:     f.Type,
			IsTime:   f.Type == domain.FieldTime,
			Required: f.Type == domain.FieldString && !f.Optional,
		}

		switch f.Type {
		case domain.FieldString:
			label := strings.ReplaceAll(f.Name, "_", " ")
			field.GoType = "string"
			field.Sample, field.Updated = strconv.Quote("example "+label), strconv.Quote("updated "+label)
			field.SampleJSON, field.UpdatedJSON = field.Sample, field.Updated
			if field.Required && data.RequiredField == "" {
				data.RequiredField = field.GoName
			}
		case domain.FieldInt:
			field.GoType, field.Sample, field.Updated = "int", "42", "7"
		case domain.FieldFloat:
			field.GoType, field.Sample, field.Updated = "float64", "9.5", "19.25"
		case domain.FieldBool:
			field.GoType, field.Sample, field.Updated = "bool", "true", "false"
		case domain.FieldTime:
			field.GoType = "time.Time"
			field.Sample = "time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)"
			field.Updated = "time.Date(2026, 2, 3, 16, 5, 6, 0, time.UTC)"
			field.SampleJSON, field.UpdatedJSON = `"2026-01-02T15:04:05Z"`, `"2026-02-03T16:05:06Z"`
		}
		if field.SampleJSON == "" {
			field.SampleJSON, field.UpdatedJSON = field.Sample, field.Updated
		}

		data.Fields = append(data.Fields, field)
	}

	data.First = data.Fields[0]
	data.SampleBody = prettyBody(data.Fields, func(f resourceField) string { return f.SampleJSON })
	data.UpdatedBody = prettyBody(data.Fields, func(f resourceField) string { return f.UpdatedJSON })
	return data
}

// prettyBody renders fields as an indented JSON object, using value for each field's JSON value.
func prettyBody(fields []resourceField, value func(resourceField) string) string {
	lines := make([]string, len(fields))
	for i, f := range fields {
		lines[i] = fmt.Sprintf("  %q: %s", f.JSON, value(f))
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n}"
}

// storage describes where a resource generated for eng keeps its data, with nil
// meaning the in-memory repository.
func storage(eng *engine) string {
	switch {
	case eng == nil:
		return "Data is stored in memory. Run `goforge add db <postgres|mysql|sqlite|mongo>` to store it in a database."
	case eng.dialect != nil:
		return "Data is stored in " + eng.label + "; its table is created by the new migration when the server starts."
	default:
		return "Data is stored in " + eng.label + "; its collection is created on the first write."
	}
}
