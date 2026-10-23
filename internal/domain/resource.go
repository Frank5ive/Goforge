package domain

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
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
	// FieldEmail is a string that must be an email address.
	FieldEmail FieldType = "email"
)

// FieldTypes lists the supported field types, for help text and errors.
var FieldTypes = []FieldType{FieldString, FieldInt, FieldFloat, FieldBool, FieldTime, FieldEmail}

// Field is one client-editable field of a resource.
type Field struct {
	Name string    `json:"name"` // snake_case, e.g. "in_stock"
	Type FieldType `json:"type"`
	// Optional string and email fields may be empty; otherwise they are required.
	Optional bool `json:"optional,omitempty"`
	// Min and Max bound a number's value, or a string's length in characters
	// (ignoring surrounding spaces). Nil means unbounded.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// IsText reports whether the field holds text (string or email).
func (f Field) IsText() bool {
	return f.Type == FieldString || f.Type == FieldEmail
}

// Required reports whether a text field must not be blank.
func (f Field) Required() bool {
	return f.IsText() && !f.Optional
}

// Spec returns the field in the --fields syntax, e.g. "name:string(1..100)" or
// "notes:string?".
func (f Field) Spec() string {
	s := f.Name + ":" + string(f.Type)
	if f.Min != nil || f.Max != nil {
		s += "(" + formatBound(f.Min) + ".." + formatBound(f.Max) + ")"
	}
	if f.Optional {
		s += "?"
	}
	return s
}

func formatBound(b *float64) string {
	if b == nil {
		return ""
	}
	return strconv.FormatFloat(*b, 'f', -1, 64)
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
		if f.Optional && !f.IsText() {
			return NewValidationErrorWithField("fields", fmt.Sprintf("field %q: only string and email fields can be optional (other types already accept their zero value)", f.Name))
		}
		if err := validateBounds(f); err != nil {
			return err
		}

		goName := CamelCase(f.Name)
		if seen[goName] {
			return NewValidationErrorWithField("fields", fmt.Sprintf("field %q is defined more than once", f.Name))
		}
		seen[goName] = true
	}

	return nil
}

// validateBounds checks a field's (min..max) range.
func validateBounds(f Field) error {
	if f.Min == nil && f.Max == nil {
		return nil
	}
	invalid := func(why string) error {
		return NewValidationErrorWithField("fields", fmt.Sprintf("field %q: %s", f.Name, why))
	}
	switch f.Type {
	case FieldString, FieldInt, FieldFloat:
	default:
		return invalid(fmt.Sprintf("a (min..max) range applies to string, int, and float fields, not %s", f.Type))
	}
	for _, b := range []*float64{f.Min, f.Max} {
		if b == nil {
			continue
		}
		if f.Type != FieldFloat && *b != math.Trunc(*b) {
			return invalid("the range must use whole numbers")
		}
		if f.Type == FieldString && *b < 0 {
			return invalid("a length can't be negative")
		}
	}
	if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
		return invalid("the range's minimum is above its maximum")
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

// ParseFields parses a field list like
// "title:string(1..100), price:float(0..), email:email, image_url:string?".
// A field without a type is a string. A "(min..max)" range bounds a number's
// value or a string's length, and either end may be left out. A trailing "?"
// makes a string or email field optional (they are required by default).
// Names may use hyphens, which become underscores.
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
			// Accept "?" before or after the range: string?(1..5) or string(1..5)?
			field.Optional = strings.Contains(typ, "?")
			typ = strings.ReplaceAll(typ, "?", "")
			if open := strings.Index(typ, "("); open >= 0 {
				min, max, err := parseRange(typ[open:])
				if err != nil {
					return nil, NewValidationErrorWithField("fields", fmt.Sprintf("field %q: %v", strings.TrimSpace(name), err))
				}
				field.Min, field.Max = min, max
				typ = strings.TrimSpace(typ[:open])
			}
			field.Type = FieldType(typ)
		}
		if field.Name == "" {
			return nil, NewValidationErrorWithField("fields", fmt.Sprintf("missing field name in %q", part))
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// ParseResourceSpec parses a resource given as "name:fields", or
// "name/plural:fields" for an irregular plural, for example
// "product:name:string(1..100),price:float(0..)" or "person/people:name:string".
// The result has no ProjectDir.
func ParseResourceSpec(spec string) (*ResourceConfig, error) {
	head, fieldSpec, ok := strings.Cut(strings.TrimSpace(spec), ":")
	if !ok || strings.TrimSpace(fieldSpec) == "" {
		return nil, NewValidationErrorWithField("resource", fmt.Sprintf("invalid resource %q: use name:fields, e.g. product:name:string,price:float", spec))
	}
	name, plural, _ := strings.Cut(head, "/")
	fields, err := ParseFields(fieldSpec)
	if err != nil {
		return nil, err
	}
	return &ResourceConfig{
		Name:   strings.ReplaceAll(strings.TrimSpace(name), "-", "_"),
		Plural: strings.ReplaceAll(strings.TrimSpace(plural), "-", "_"),
		Fields: fields,
	}, nil
}

// parseRange parses "(min..max)", where either bound may be empty.
func parseRange(s string) (*float64, *float64, error) {
	inner, ok := strings.CutPrefix(s, "(")
	inner, closed := strings.CutSuffix(inner, ")")
	lo, hi, dots := strings.Cut(inner, "..")
	if !ok || !closed || !dots {
		return nil, nil, fmt.Errorf("invalid range %q: use (min..max), (min..), or (..max)", s)
	}
	bound := func(v string) (*float64, error) {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, nil
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return nil, fmt.Errorf("invalid range %q: %q is not a number", s, v)
		}
		return &n, nil
	}
	min, err := bound(lo)
	if err != nil {
		return nil, nil, err
	}
	max, err := bound(hi)
	if err != nil {
		return nil, nil, err
	}
	if min == nil && max == nil {
		return nil, nil, fmt.Errorf("invalid range %q: give a minimum, a maximum, or both", s)
	}
	return min, max, nil
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
