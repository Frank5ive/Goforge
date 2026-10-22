package usecase

import (
	"errors"
	"fmt"
	"go/format"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
}

// NewResourceGeneratorUsecase creates a new instance of ResourceGeneratorUsecase.
func NewResourceGeneratorUsecase(
	renderer domain.TemplateRenderer,
	writer domain.FileWriter,
) *ResourceGeneratorUsecase {
	return &ResourceGeneratorUsecase{
		templateRenderer: renderer,
		fileWriter:       writer,
	}
}

// Add implements domain.ResourceGenerator. Everything that can be checked is
// checked before any file is written, and if a later step fails the new
// resource package is removed, so a failed run leaves the project unchanged.
func (u *ResourceGeneratorUsecase) Add(config *domain.ResourceConfig) error {
	if err := config.Validate(); err != nil {
		return domain.NewGenerationError("validation", "resource validation failed", err)
	}

	modulePath, err := u.readModulePath(config.ProjectDir)
	if err != nil {
		return domain.NewGenerationError("project_check",
			fmt.Sprintf("%s is not a Go module; run this inside a project created by goforge generate", config.ProjectDir), err)
	}

	manifest, hasManifest, err := readManifest(u.fileWriter, config.ProjectDir)
	if err != nil {
		return domain.NewGenerationError("project_check", "cannot read "+domain.ManifestFile, err)
	}
	if !hasManifest {
		// Projects from older GoForge versions have no manifest; start one
		manifest = domain.Manifest{Module: modulePath}
	}

	routesPath := filepath.Join(config.ProjectDir, "server", "routes.go")
	routes, err := u.fileWriter.ReadFile(routesPath)
	if err != nil {
		return domain.NewGenerationError("project_check",
			"server/routes.go not found; run this inside a project created by goforge generate", err)
	}

	updatedRoutes, err := registerResource(routes, modulePath, config.PackageName())
	if err != nil {
		return domain.NewGenerationError("register_routes", "cannot register the resource in server/routes.go", err)
	}

	pkgDir := filepath.Join(config.ProjectDir, config.PackageName())
	if u.fileWriter.FileExists(pkgDir) {
		return domain.NewGenerationError("directory_check", fmt.Sprintf("%s already exists", pkgDir), nil)
	}

	if err := u.fileWriter.CreateDirectory(pkgDir); err != nil {
		return domain.NewGenerationError("create_structure", "failed to create the resource package directory", err)
	}

	data := newResourceData(config, modulePath)
	data.Port = u.readDefaultPort(config.ProjectDir)
	for _, f := range resourceFiles {
		content, err := u.templateRenderer.Render(f.template, data)
		if err == nil {
			err = writeFormatted(u.fileWriter, filepath.Join(pkgDir, f.file(config)), content)
		}
		if err != nil {
			return u.cleanup(pkgDir, domain.NewGenerationError("generate_files", fmt.Sprintf("failed to generate %s", f.file(config)), err))
		}
	}

	if err := u.fileWriter.WriteFile(routesPath, updatedRoutes); err != nil {
		return u.cleanup(pkgDir, domain.NewGenerationError("register_routes", "failed to update server/routes.go", err))
	}

	manifest.AddResource(config)
	if err := writeManifest(u.fileWriter, config.ProjectDir, manifest); err != nil {
		// The resource itself is complete and wired in, so keep it
		return domain.NewGenerationError("manifest",
			fmt.Sprintf("the resource was added, but %s could not be updated", domain.ManifestFile), err)
	}

	return nil
}

// cleanup removes the partially generated resource package and returns genErr,
// noting a cleanup failure if there was one.
func (u *ResourceGeneratorUsecase) cleanup(pkgDir string, genErr *domain.GenerationError) error {
	if err := u.fileWriter.RemoveAll(pkgDir); err != nil {
		genErr.Message += fmt.Sprintf(" (cleanup of %s also failed: %v)", pkgDir, err)
	}
	return genErr
}

// readModulePath returns the module path declared in the project's go.mod.
func (u *ResourceGeneratorUsecase) readModulePath(projectDir string) (string, error) {
	goMod, err := u.fileWriter.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(goMod, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	return "", errors.New("go.mod has no module directive")
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
// and its routes registered just above the goforge:routes marker.
func registerResource(src, modulePath, pkg string) (string, error) {
	importPath := strconv.Quote(modulePath + "/" + pkg)
	registration := fmt.Sprintf("%s.RegisterRoutes(mux, %s.NewService(%s.NewMemoryRepository()))", pkg, pkg, pkg)

	if strings.Contains(src, importPath) {
		return "", fmt.Errorf("%s is already imported", importPath)
	}

	markerAt := strings.Index(src, routesMarker)
	if markerAt < 0 {
		return "", fmt.Errorf("the %q marker is missing; add it inside registerRoutes, or register the resource yourself with:\n\t%s", routesMarker, registration)
	}
	lineStart := strings.LastIndex(src[:markerAt], "\n") + 1
	indent := src[lineStart:markerAt]
	src = src[:lineStart] + indent + registration + "\n" + src[lineStart:]

	importStart := strings.Index(src, "import (")
	if importStart < 0 {
		return "", errors.New("no import block found")
	}
	importEnd := strings.Index(src[importStart:], "\n)")
	if importEnd < 0 {
		return "", errors.New("unterminated import block")
	}
	importEnd += importStart
	src = src[:importEnd] + "\n\t" + importPath + src[importEnd:]

	formatted, err := format.Source([]byte(src))
	if err != nil {
		return "", fmt.Errorf("updated routes.go is not valid Go: %w", err)
	}
	return string(formatted), nil
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
}

// resourceField describes one field for the templates, including sample values
// used by the generated tests.
type resourceField struct {
	GoName      string
	JSON        string
	GoType      string
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
