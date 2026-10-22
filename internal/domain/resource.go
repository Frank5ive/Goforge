package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// FieldType is the type of a resource field.
type FieldType string

// Supported field types.
const (
	FieldString FieldType = "string"
	FieldInt    FieldType = "int"
	FieldFloat  FieldType = "float"
	FieldBool   FieldType = "bool"
	FieldTime   FieldType = "time"
)

// FieldTypes lists the supported field types, for help text and errors.
var FieldTypes = []FieldType{FieldString, FieldInt, FieldFloat, FieldBool, FieldTime}

// Field is one client-editable field of a resource.
type Field struct {
	Name string    `json:"name"` // snake_case, e.g. "in_stock"
	Type FieldType `json:"type"`
	// Optional string fields may be empty; other string fields are required.
	Optional bool `json:"optional,omitempty"`
}

// ResourceConfig describes a REST resource to add to an existing GoForge project.
type ResourceConfig struct {
	// ProjectDir is the root of the project (the directory containing go.mod).
	ProjectDir string
	// Name is the singular resource name in snake_case, e.g. "blog_post".
	Name string
	// Plural overrides the derived plural name, e.g. "people" for "person".
	Plural string
	// Fields are the client-editable fields. ID and timestamps are added automatically.
	Fields []Field
}

// identifierPattern matches snake_case identifiers: lowercase words separated by
// single underscores, starting with a letter.
var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// reservedPackages are directories a generated project already uses, so a
// resource package cannot take their names.
var reservedPackages = []string{"bin", "config", "docs", "handlers", "internal", "logger", "main", "server", "testdata", "vendor"}

// reservedResourceNames would collide with the types generated in every resource
// package (Service, Input, Repository, ...).
var reservedResourceNames = []string{"handler", "input", "memory_repository", "repository", "service"}

// reservedFields are added to every resource automatically.
var reservedFields = []string{"created_at", "id", "updated_at"}

var goKeywords = []string{
	"break", "case", "chan", "const", "continue", "default", "defer", "else",
	"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
	"map", "package", "range", "return", "select", "struct", "switch", "type", "var",
}

// Validate checks the resource name, plural, and fields.
func (c *ResourceConfig) Validate() error {
	if strings.TrimSpace(c.ProjectDir) == "" {
		return NewValidationErrorWithField("dir", "project directory cannot be empty")
	}

	if !identifierPattern.MatchString(c.Name) {
		return NewValidationErrorWithField("name", fmt.Sprintf("invalid resource name %q: use singular snake_case, like product or blog_post", c.Name))
	}
	if slices.Contains(goKeywords, c.Name) || slices.Contains(reservedResourceNames, c.Name) {
		return NewValidationErrorWithField("name", fmt.Sprintf("resource name %q is reserved", c.Name))
	}

	plural := c.PluralName()
	if !identifierPattern.MatchString(plural) {
		return NewValidationErrorWithField("plural", fmt.Sprintf("invalid plural %q: use snake_case, like blog_posts", plural))
	}
	if plural == c.Name {
		return NewValidationErrorWithField("plural", fmt.Sprintf("plural %q must differ from the resource name", plural))
	}
	if pkg := c.PackageName(); slices.Contains(goKeywords, pkg) || slices.Contains(reservedPackages, pkg) {
		return NewValidationErrorWithField("plural", fmt.Sprintf("package name %q is reserved; choose another name or --plural", pkg))
	}

	return validateFields(c.Fields)
}

func validateFields(fields []Field) error {
	if len(fields) == 0 {
		return NewValidationErrorWithField("fields", "a resource needs at least one field")
	}

	seen := map[string]bool{}
	for _, f := range fields {
		if !identifierPattern.MatchString(f.Name) {
			return NewValidationErrorWithField("fields", fmt.Sprintf("invalid field name %q: use snake_case, like in_stock", f.Name))
		}
		if slices.Contains(reservedFields, f.Name) {
			return NewValidationErrorWithField("fields", fmt.Sprintf("field %q is added automatically; remove it from --fields", f.Name))
		}
		if !slices.Contains(FieldTypes, f.Type) {
			return NewValidationErrorWithField("fields", fmt.Sprintf("field %q has unsupported type %q (supported: %s)", f.Name, f.Type, fieldTypeList()))
		}
		if f.Optional && f.Type != FieldString {
			return NewValidationErrorWithField("fields", fmt.Sprintf("field %q: only string fields can be optional (other types already accept their zero value)", f.Name))
		}

		goName := CamelCase(f.Name)
		if seen[goName] {
			return NewValidationErrorWithField("fields", fmt.Sprintf("field %q is defined more than once", f.Name))
		}
		seen[goName] = true
	}

	return nil
}

// PluralName returns Plural if set, otherwise the derived plural of Name.
func (c *ResourceConfig) PluralName() string {
	if c.Plural != "" {
		return c.Plural
	}
	return Pluralize(c.Name)
}

// PackageName is the Go package (and directory) for the resource, e.g. "blogposts".
func (c *ResourceConfig) PackageName() string {
	return strings.ReplaceAll(c.PluralName(), "_", "")
}

// RoutePath is the URL path for the collection, e.g. "/blog-posts".
func (c *ResourceConfig) RoutePath() string {
	return "/" + strings.ReplaceAll(c.PluralName(), "_", "-")
}

// ParseFields parses a field list like "title:string, price:float, image_url:string?".
// A field without a type is a string, and a trailing "?" marks a string field as
// optional (string fields are required by default). Names may use hyphens, which
// become underscores.
func ParseFields(spec string) ([]Field, error) {
	var fields []Field

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, typ, hasType := strings.Cut(part, ":")
		field := Field{
			Name: strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "-", "_"),
			Type: FieldString,
		}
		if hasType {
			typ = strings.ToLower(strings.TrimSpace(typ))
			field.Optional = strings.HasSuffix(typ, "?")
			field.Type = FieldType(strings.TrimSuffix(typ, "?"))
		}
		if field.Name == "" {
			return nil, NewValidationErrorWithField("fields", fmt.Sprintf("missing field name in %q", part))
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// Pluralize returns the English plural of a snake_case word using simple rules:
// "category" → "categories", "box" → "boxes", "product" → "products".
// Use ResourceConfig.Plural for irregular words like "person".
func Pluralize(word string) string {
	switch {
	case len(word) > 1 && strings.HasSuffix(word, "y") && !strings.ContainsRune("aeiou", rune(word[len(word)-2])):
		return word[:len(word)-1] + "ies"
	case strings.HasSuffix(word, "s"), strings.HasSuffix(word, "x"), strings.HasSuffix(word, "z"),
		strings.HasSuffix(word, "ch"), strings.HasSuffix(word, "sh"):
		return word + "es"
	default:
		return word + "s"
	}
}

// initialisms are written in all caps in Go names, per Go naming conventions.
var initialisms = map[string]string{
	"api": "API", "html": "HTML", "http": "HTTP", "id": "ID", "ip": "IP",
	"json": "JSON", "sku": "SKU", "sql": "SQL", "url": "URL", "uuid": "UUID",
}

// CamelCase converts snake_case to an exported Go name: "image_url" → "ImageURL".
func CamelCase(snake string) string {
	var b strings.Builder
	for _, word := range strings.Split(snake, "_") {
		if word == "" {
			continue
		}
		if upper, ok := initialisms[word]; ok {
			b.WriteString(upper)
			continue
		}
		b.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return b.String()
}

func fieldTypeList() string {
	names := make([]string, len(FieldTypes))
	for i, t := range FieldTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
