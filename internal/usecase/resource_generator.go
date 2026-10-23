package usecase

import (
	"fmt"
	"math"
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
	{"resource/rules.tmpl", func(*domain.ResourceConfig) string { return "rules.go" }},
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
		return domain.Changes{}, domain.WithCode(domain.CodeNotAProject, domain.NewGenerationError("project_check",
			fmt.Sprintf("%s is not a Go module; run this inside a project created by goforge generate", dir), err))
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
		return domain.Changes{}, domain.WithCode(domain.CodeNotAProject, domain.NewGenerationError("project_check",
			"server/routes.go not found; run this inside a project created by goforge generate", err))
	}

	pkg := config.PackageName()
	repo := pkg + ".NewMemoryRepository()"
	if eng != nil {
		repo = fmt.Sprintf("%s.New%s(deps.DB)", pkg, eng.repoType)
	}
	refs, err := resolveRefs(config, manifest)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("validation", "resource validation failed", err)
	}
	updatedRoutes, err := registerResource(routes, modulePath, pkg, repo, refs)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("register_routes", "cannot register the resource in server/routes.go", err)
	}

	pkgDir := filepath.Join(dir, pkg)
	if u.fileWriter.FileExists(pkgDir) {
		return domain.Changes{}, domain.WithCode(domain.CodeAlreadyExists,
			domain.NewGenerationError("directory_check", fmt.Sprintf("%s already exists", pkgDir), nil))
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
func registerResource(src, modulePath, pkg, repo string, refs []routeRef) (string, error) {
	importPath := modulePath + "/" + pkg
	registration := fmt.Sprintf("%s.RegisterRoutes(mux, %s.NewService(%s))", pkg, pkg, repo)

	if strings.Contains(src, strconv.Quote(importPath)) {
		return "", domain.WithCode(domain.CodeAlreadyExists, fmt.Errorf("%q is already imported", importPath))
	}

	// A resource with references gets a service variable, so it can be wired to
	// the services it refers to, which need variables too
	if len(refs) > 0 {
		service := pkg + "Service"
		lines := []string{fmt.Sprintf("%s := %s.NewService(%s)", service, pkg, repo)}
		for _, ref := range refs {
			target := ref.TargetPkg + "Service"
			if ref.TargetPkg != pkg {
				var err error
				if src, err = ensureServiceVar(src, ref.TargetPkg); err != nil {
					return "", err
				}
			}
			lines = append(lines,
				fmt.Sprintf("%s.Require%s(%s.Exists)", service, ref.GoName, target),
				fmt.Sprintf("%s.BlockDeleteWhen(%q, %s.Has%s)", target, ref.Owner, service, ref.GoName))
		}
		registration = strings.Join(append(lines, fmt.Sprintf("%s.RegisterRoutes(mux, %s)", pkg, service)), "\n")
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

// routeRef is a reference from the resource being added to another resource,
// for wiring their services in server/routes.go.
type routeRef struct {
	GoName    string // the field's Go name, e.g. "CategoryID"
	TargetPkg string // the referenced resource's package, e.g. "categories"
	Owner     string // what refers to it, for the 409 message, e.g. "products"
}

// resolveRefs checks that every ref field points to a resource that exists (or
// to the resource itself), and returns how to wire them.
func resolveRefs(config *domain.ResourceConfig, manifest domain.Manifest) ([]routeRef, error) {
	var refs []routeRef
	for _, f := range config.Fields {
		if f.Type != domain.FieldRef {
			continue
		}
		ref := routeRef{GoName: domain.CamelCase(f.Name), Owner: strings.ReplaceAll(config.PluralName(), "_", " ")}
		if f.Ref == config.Name {
			ref.TargetPkg = config.PackageName()
		}
		for _, r := range manifest.Resources {
			if r.Name == f.Ref {
				ref.TargetPkg = r.ResourceConfig(config.ProjectDir).PackageName()
			}
		}
		if ref.TargetPkg == "" {
			return nil, domain.NewValidationErrorWithField("fields",
				fmt.Sprintf("field %q refers to %s, which isn't a resource in this project yet; add it first", f.Name, f.Ref))
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// ensureServiceVar makes sure routes.go keeps pkg's service in a variable
// (pkgService), rewriting a one-line registration if needed, so other services
// can refer to it.
func ensureServiceVar(src, pkg string) (string, error) {
	service := pkg + "Service"
	// Indentation is spaces and tabs only, so a match never starts on a blank line
	if regexp.MustCompile(`(?m)^[ \t]*` + service + ` := `).MatchString(src) {
		return src, nil
	}
	oneLine := regexp.MustCompile(`(?m)^([ \t]*)` + pkg + `\.RegisterRoutes\(mux, (` + pkg + `\.NewService\(.*\))\)[ \t]*$`)
	loc := oneLine.FindStringSubmatchIndex(src)
	if loc == nil {
		return "", domain.WithCode(domain.CodeMarkerMissing, fmt.Errorf(
			"server/routes.go doesn't register %s the way goforge add resource does; keep its service in a variable named %s, then run the command again", pkg, service))
	}
	indent, newService := src[loc[2]:loc[3]], src[loc[4]:loc[5]]
	rewritten := fmt.Sprintf("%s%s := %s\n%s%s.RegisterRoutes(mux, %s)", indent, service, newService, indent, pkg, service)
	return src[:loc[0]] + rewritten + src[loc[1]:], nil
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
	// there is none.
	RequiredField string
	// Validation needs these imports in the model (strings, unicode/utf8, and
	// a validEmail helper) and tests (strings, for long values)
	UsesStrings, UsesUTF8, UsesEmail, TestsUseStrings bool
	// InvalidCases are values that break one rule each, for the generated tests
	InvalidCases []invalidCase
	// Refs are the resource's ref fields
	Refs        []refData
	Port        int    // the project's default port, for requests.http
	SampleBody  string // pretty-printed JSON create request, for requests.http
	UpdatedBody string // pretty-printed JSON replace request, for requests.http

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
	Required    bool   // a required string or email field
	Sample      string // Go literal
	Updated     string // a different Go literal
	SampleJSON  string // Sample as JSON
	UpdatedJSON string // Updated as JSON
	// Checks are the field's validation rules, in Input.Validate
	Checks []fieldCheck
}

// refData describes a ref field for the templates.
type refData struct {
	GoName      string // e.g. "CategoryID"
	JSON        string // e.g. "category_id"
	Optional    bool
	TargetHuman string // the referenced resource, e.g. "category"
	ExistsVar   string // the service field holding the existence check, e.g. "categoryIDExists"
	Column      string // quoted SQL column, set by addEngineData
	CountGo     string // SQL counting the items that refer to an ID, as a Go literal; set by addEngineData
}

// fieldCheck is one validation rule: Cond (a Go if condition, possibly with an
// init statement) is true when the value breaks the rule, and Message (a Go
// string literal) says why.
type fieldCheck struct {
	Cond, Message string
}

// invalidCase is a value that breaks one of a field's rules.
type invalidCase struct {
	Name  string // Go string literal naming the test case
	Field string // Go field name
	Value string // Go expression
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
		// Email and ref fields are stored as text: only their checks differ
		storageType := f.Type
		if f.Type == domain.FieldEmail || f.Type == domain.FieldRef {
			storageType = domain.FieldString
		}
		field := resourceField{
			GoName:   domain.CamelCase(f.Name),
			JSON:     f.Name,
			Type:     storageType,
			IsTime:   f.Type == domain.FieldTime,
			Required: f.Required(),
		}

		switch f.Type {
		case domain.FieldString:
			label := strings.ReplaceAll(f.Name, "_", " ")
			field.GoType = "string"
			field.Sample = strconv.Quote(fitLength("example "+label, f))
			field.Updated = strconv.Quote(fitLength("updated "+label, f))
		case domain.FieldEmail:
			field.GoType = "string"
			field.Sample, field.Updated = `"a@example.com"`, `"b@example.com"`
		case domain.FieldRef:
			field.GoType = "string"
			field.Sample, field.Updated = strconv.Quote(f.Ref+"-1"), strconv.Quote(f.Ref+"-2")
			data.Refs = append(data.Refs, refData{
				GoName: field.GoName, JSON: f.Name, Optional: f.Optional,
				TargetHuman: strings.ReplaceAll(f.Ref, "_", " "),
				ExistsVar:   strings.ToLower(field.GoName[:1]) + field.GoName[1:] + "Exists",
			})
		case domain.FieldInt:
			field.GoType = "int"
			sample, updated := fitNumber(42, 7, f, true)
			field.Sample, field.Updated = formatNumber(sample), formatNumber(updated)
		case domain.FieldFloat:
			field.GoType = "float64"
			sample, updated := fitNumber(9.5, 19.25, f, false)
			field.Sample, field.Updated = formatNumber(sample), formatNumber(updated)
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
		if field.Required && data.RequiredField == "" {
			data.RequiredField = field.GoName
		}

		var invalid []invalidCase
		field.Checks, invalid = fieldRules(f, field.GoName)
		data.InvalidCases = append(data.InvalidCases, invalid...)
		for _, c := range field.Checks {
			data.UsesStrings = data.UsesStrings || strings.Contains(c.Cond, "strings.")
			data.UsesUTF8 = data.UsesUTF8 || strings.Contains(c.Cond, "utf8.")
			data.UsesEmail = data.UsesEmail || strings.Contains(c.Cond, "validEmail(")
		}
		for _, c := range invalid {
			data.TestsUseStrings = data.TestsUseStrings || strings.Contains(c.Value, "strings.")
		}

		data.Fields = append(data.Fields, field)
	}

	data.First = data.Fields[0]
	data.SampleBody = prettyBody(data.Fields, func(f resourceField) string { return f.SampleJSON })
	data.UpdatedBody = prettyBody(data.Fields, func(f resourceField) string { return f.UpdatedJSON })
	return data
}

// fieldRules returns the validation checks for a field, and a value that
// breaks each one, for the generated tests.
func fieldRules(f domain.Field, goName string) ([]fieldCheck, []invalidCase) {
	var checks []fieldCheck
	var invalid []invalidCase
	value := "in." + goName
	add := func(cond, message, name, bad string) {
		checks = append(checks, fieldCheck{Cond: cond, Message: strconv.Quote(message)})
		if bad != "" {
			invalid = append(invalid, invalidCase{Name: strconv.Quote(name), Field: goName, Value: bad})
		}
	}
	// guard skips a rule for an optional field left empty
	guard := func(cond string) string {
		if f.Optional {
			return value + ` != "" && ` + cond
		}
		return cond
	}

	if f.Required() {
		add(`strings.TrimSpace(`+value+`) == ""`, f.Name+" is required", f.Name+" is blank", `" "`)
	}
	switch f.Type {
	case domain.FieldEmail:
		add(guard("!validEmail("+value+")"), f.Name+" must be an email address", f.Name+" is not an email address", `"not-an-email"`)
	case domain.FieldString:
		if f.Min == nil && f.Max == nil {
			break
		}
		length := "n := utf8.RuneCountInString(strings.TrimSpace(" + value + ")); "
		if f.Optional {
			length += "n > 0 && "
		}
		var cond []string
		if f.Min != nil {
			cond = append(cond, "n < "+formatNumber(*f.Min))
			// A required field's blank value already covers a minimum of 1
			if *f.Min > 1 {
				invalid = append(invalid, invalidCase{Name: strconv.Quote(f.Name + " is too short"), Field: goName,
					Value: fmt.Sprintf("%q", strings.Repeat("x", int(*f.Min)-1))})
			}
		}
		if f.Max != nil {
			cond = append(cond, "n > "+formatNumber(*f.Max))
			invalid = append(invalid, invalidCase{Name: strconv.Quote(f.Name + " is too long"), Field: goName,
				Value: fmt.Sprintf("strings.Repeat(\"x\", %d)", int(*f.Max)+1)})
		}
		check := length + strings.Join(cond, " || ")
		if f.Optional && len(cond) > 1 {
			check = length + "(" + strings.Join(cond, " || ") + ")"
		}
		message := f.Name + " must be " + describeRange(f, " characters")
		checks = append(checks, fieldCheck{Cond: check, Message: strconv.Quote(message)})
	case domain.FieldInt, domain.FieldFloat:
		if f.Min != nil {
			add(value+" < "+formatNumber(*f.Min), f.Name+" must be at least "+formatNumber(*f.Min), f.Name+" is below the minimum", formatNumber(*f.Min-1))
		}
		if f.Max != nil {
			add(value+" > "+formatNumber(*f.Max), f.Name+" must be at most "+formatNumber(*f.Max), f.Name+" is above the maximum", formatNumber(*f.Max+1))
		}
	}
	return checks, invalid
}

// describeRange describes a string length range, e.g. "1 to 100 characters".
func describeRange(f domain.Field, unit string) string {
	switch {
	case f.Min != nil && f.Max != nil:
		return formatNumber(*f.Min) + " to " + formatNumber(*f.Max) + unit
	case f.Min != nil:
		return "at least " + formatNumber(*f.Min) + unit
	default:
		return "at most " + formatNumber(*f.Max) + unit
	}
}

// fitLength trims or pads a sample string to fit the field's length range.
func fitLength(s string, f domain.Field) string {
	if f.Max != nil && len(s) > int(*f.Max) {
		s = s[:int(*f.Max)]
	}
	if f.Min != nil && len(strings.TrimSpace(s)) < int(*f.Min) {
		s = strings.TrimSpace(s) + strings.Repeat("x", int(*f.Min)-len(strings.TrimSpace(s)))
	}
	return s
}

// fitNumber moves the sample and updated values into the field's range,
// keeping them different when the range allows.
func fitNumber(sample, updated float64, f domain.Field, whole bool) (float64, float64) {
	lo, hi := math.Inf(-1), math.Inf(1)
	if f.Min != nil {
		lo = *f.Min
	}
	if f.Max != nil {
		hi = *f.Max
	}
	clamp := func(v float64) float64 { return math.Min(math.Max(v, lo), hi) }
	sample, updated = clamp(sample), clamp(updated)
	if sample == updated {
		step := 1.0
		if !whole {
			step = 0.5
		}
		if updated+step <= hi {
			updated += step
		} else if updated-step >= lo {
			updated -= step
		}
	}
	return sample, updated
}

// formatNumber writes a number as a Go and JSON literal.
func formatNumber(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
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
