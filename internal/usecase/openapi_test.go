package usecase

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

func bound(v float64) *float64 { return &v }

func shopManifest() domain.Manifest {
	return domain.Manifest{
		AppName: "shop", Module: "example.com/shop", Port: 3000,
		Resources: []domain.ManifestResource{
			{Name: "category", Plural: "categories", Fields: []domain.Field{
				{Name: "name", Type: domain.FieldString},
				{Name: "notes", Type: domain.FieldString, Optional: true},
			}},
			{Name: "blog_post", Plural: "blog_posts", Fields: []domain.Field{
				{Name: "title", Type: domain.FieldString, Min: bound(1), Max: bound(100)},
				{Name: "contact", Type: domain.FieldEmail},
				{Name: "stock", Type: domain.FieldInt, Min: bound(0), Max: bound(1000)},
				{Name: "price", Type: domain.FieldFloat, Min: bound(0)},
				{Name: "live", Type: domain.FieldBool},
				{Name: "published_at", Type: domain.FieldTime},
				{Name: "category_id", Type: domain.FieldRef, Ref: "category"},
			}},
		},
	}
}

func parseSpec(t *testing.T, m domain.Manifest) map[string]any {
	t.Helper()
	content, err := buildOpenAPI(m)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatalf("the document is not valid JSON: %v\n%s", err, content)
	}
	return doc
}

// at walks a decoded document by keys.
func at(t *testing.T, doc any, keys ...string) any {
	t.Helper()
	for _, k := range keys {
		m, ok := doc.(map[string]any)
		if !ok {
			t.Fatalf("%v: not an object at %q", keys, k)
		}
		if doc, ok = m[k]; !ok {
			t.Fatalf("%v: no key %q", keys, k)
		}
	}
	return doc
}

func TestBuildOpenAPI_DescribesTheProject(t *testing.T) {
	doc := parseSpec(t, shopManifest())

	if at(t, doc, "openapi") != "3.1.0" || at(t, doc, "info", "title") != "shop API" {
		t.Errorf("openapi/info = %v / %v", doc["openapi"], doc["info"])
	}
	if at(t, doc, "servers").([]any)[0].(map[string]any)["url"] != "http://localhost:3000" {
		t.Errorf("servers = %v, want the default port", doc["servers"])
	}

	paths := at(t, doc, "paths").(map[string]any)
	for _, p := range []string{"/health", "/readyz", "/openapi.json", "/categories", "/categories/{id}", "/blog-posts", "/blog-posts/{id}"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("paths lacks %s: %v", p, keysOf(paths))
		}
	}
	collection := at(t, doc, "paths", "/blog-posts").(map[string]any)
	item := at(t, doc, "paths", "/blog-posts/{id}").(map[string]any)
	if keysOf(collection) != "get,post" || keysOf(item) != "delete,get,patch,put" {
		t.Errorf("methods = %s and %s, want get,post and delete,get,patch,put", keysOf(collection), keysOf(item))
	}
}

func keysOf(m map[string]any) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	// json.Unmarshal's map order is random
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return strings.Join(keys, ",")
}

func TestBuildOpenAPI_FieldRulesBecomeSchemaRules(t *testing.T) {
	doc := parseSpec(t, shopManifest())
	props := func(schema string) map[string]any {
		return at(t, doc, "components", "schemas", schema, "properties").(map[string]any)
	}

	title := props("BlogPostInput")["title"].(map[string]any)
	if title["type"] != "string" || title["minLength"] != 1.0 || title["maxLength"] != 100.0 {
		t.Errorf("title = %v, want a string of 1 to 100 characters", title)
	}
	stock := props("BlogPostInput")["stock"].(map[string]any)
	if stock["type"] != "integer" || stock["minimum"] != 0.0 || stock["maximum"] != 1000.0 {
		t.Errorf("stock = %v, want an integer from 0 to 1000", stock)
	}
	price := props("BlogPostInput")["price"].(map[string]any)
	if price["type"] != "number" || price["minimum"] != 0.0 || price["maximum"] != nil {
		t.Errorf("price = %v, want a number of at least 0", price)
	}
	if c := props("BlogPostInput")["contact"].(map[string]any); c["format"] != "email" {
		t.Errorf("contact = %v, want format email", c)
	}
	if p := props("BlogPostInput")["published_at"].(map[string]any); p["format"] != "date-time" {
		t.Errorf("published_at = %v, want format date-time", p)
	}
	if l := props("BlogPostInput")["live"].(map[string]any); l["type"] != "boolean" {
		t.Errorf("live = %v, want a boolean", l)
	}
	if r := props("BlogPostInput")["category_id"].(map[string]any); r["type"] != "string" || !strings.Contains(r["description"].(string), "category") {
		t.Errorf("category_id = %v, want a string that names the category", r)
	}

	// Only required text fields are required; numbers and booleans accept zero values
	required := at(t, doc, "components", "schemas", "BlogPostInput", "required").([]any)
	if len(required) != 3 || required[0] != "title" || required[1] != "contact" || required[2] != "category_id" {
		t.Errorf("required = %v, want title, contact, category_id", required)
	}
	optionalOnly := at(t, doc, "components", "schemas", "CategoryInput", "required").([]any)
	if len(optionalOnly) != 1 || optionalOnly[0] != "name" {
		t.Errorf("CategoryInput required = %v, want only name (notes is optional)", optionalOnly)
	}
	if _, patchRequires := at(t, doc, "components", "schemas", "BlogPostPatch").(map[string]any)["required"]; patchRequires {
		t.Error("a patch must not require any field")
	}
	if at(t, doc, "components", "schemas", "BlogPostInput", "additionalProperties") != false {
		t.Error("input must reject unknown fields, as the generated handlers do")
	}
}

func TestBuildOpenAPI_ResponsesFollowTheHandlers(t *testing.T) {
	doc := parseSpec(t, shopManifest())
	statuses := func(path, method string) string {
		return keysOf(at(t, doc, "paths", path, method, "responses").(map[string]any))
	}

	if got := statuses("/blog-posts", "post"); got != "201,400,413,422,500" {
		t.Errorf("POST responses = %s", got)
	}
	if at(t, doc, "paths", "/blog-posts", "post", "responses", "201", "headers", "Location") == nil {
		t.Error("a create must document its Location header")
	}
	// A category is referenced by blog posts, so deleting one can be a 409; a blog post is not
	if got := statuses("/categories/{id}", "delete"); got != "204,404,409,500" {
		t.Errorf("DELETE category responses = %s, want a 409", got)
	}
	if got := statuses("/blog-posts/{id}", "delete"); got != "204,404,500" {
		t.Errorf("DELETE blog post responses = %s, want no 409", got)
	}
	if got := statuses("/readyz", "get"); got != "200,503" {
		t.Errorf("GET /readyz responses = %s", got)
	}

	limit := at(t, doc, "paths", "/categories", "get", "parameters").([]any)[0].(map[string]any)
	if limit["name"] != "limit" || at(t, limit, "schema", "maximum") != 100.0 || at(t, limit, "schema", "default") != 20.0 {
		t.Errorf("limit parameter = %v, want default 20 and maximum 100", limit)
	}
}

func TestBuildOpenAPI_IsStableAndOptional(t *testing.T) {
	first, err := buildOpenAPI(shopManifest())
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		again, _ := buildOpenAPI(shopManifest())
		if !bytes.Equal(first, again) {
			t.Fatal("the document changes between runs, which would churn diffs")
		}
	}

	// A new project, with no resources and no port, still has a valid document
	empty := parseSpec(t, domain.Manifest{Module: "example.com/app"})
	if at(t, empty, "info", "title") != "example.com/app API" {
		t.Errorf("title = %v, want the module path without an app name", at(t, empty, "info", "title"))
	}
	if _, hasServers := empty["servers"]; hasServers {
		t.Error("a manifest without a port should not invent a server")
	}
	if got := keysOf(at(t, empty, "paths").(map[string]any)); got != "/health,/openapi.json,/readyz" {
		t.Errorf("paths = %s", got)
	}
}

func TestResourceGenerator_AddKeepsTheOpenAPIDocumentCurrent(t *testing.T) {
	writer := newProjectWriter()
	spec := filepath.Join("/proj", filepath.FromSlash(OpenAPIFile))
	writer.FilesWritten[spec] = "{}\n"
	uc := NewResourceGeneratorUsecase(NewMockTemplateRenderer(), writer)

	changes, err := uc.Add(testResource())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if !strings.Contains(writer.FilesWritten[spec], "/blog-posts") || !strings.Contains(writer.FilesWritten[spec], `"BlogPost"`) {
		t.Errorf("the document was not regenerated:\n%s", writer.FilesWritten[spec])
	}
	var updated bool
	for _, f := range changes.Updated {
		updated = updated || f == filepath.ToSlash(OpenAPIFile)
	}
	if !updated {
		t.Errorf("changes.Updated = %v, want %s listed", changes.Updated, OpenAPIFile)
	}
}

func TestResourceGenerator_AddLeavesProjectsWithoutADocumentAlone(t *testing.T) {
	writer := newProjectWriter() // generated before the OpenAPI document existed
	uc := NewResourceGeneratorUsecase(NewMockTemplateRenderer(), writer)

	if _, err := uc.Add(testResource()); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, created := writer.FilesWritten[filepath.Join("/proj", filepath.FromSlash(OpenAPIFile))]; created {
		t.Error("an older project got an OpenAPI document that nothing serves")
	}
}

func TestBuildOpenAPI_RefusesSchemaNamesThatCollide(t *testing.T) {
	str := []domain.Field{{Name: "name", Type: domain.FieldString}}
	tests := []struct {
		name      string
		resources []domain.ManifestResource
		schema    string
	}{
		{"a resource called error", []domain.ManifestResource{{Name: "error", Plural: "errors", Fields: str}}, "Error"},
		{"a resource named like another's page", []domain.ManifestResource{
			{Name: "product", Plural: "products", Fields: str},
			{Name: "product_page", Plural: "product_pages", Fields: str},
		}, "ProductPage"},
		{"a resource named like another's input", []domain.ManifestResource{
			{Name: "order", Plural: "orders", Fields: str},
			{Name: "order_input", Plural: "order_inputs", Fields: str},
		}, "OrderInput"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildOpenAPI(domain.Manifest{Module: "example.com/app", Resources: tt.resources})
			if err == nil || !strings.Contains(err.Error(), tt.schema) || !strings.Contains(err.Error(), "rename the resource") {
				t.Fatalf("buildOpenAPI() error = %v, want it to name %s and say to rename", err, tt.schema)
			}
			if code := domain.ErrorCode(err); code != domain.CodeInvalidInput {
				t.Errorf("error code = %s, want %s", code, domain.CodeInvalidInput)
			}
		})
	}
}

func TestBuildOpenAPI_SchemasAllowWhatValidateAllows(t *testing.T) {
	doc := parseSpec(t, domain.Manifest{Module: "example.com/app", Resources: []domain.ManifestResource{{
		Name: "note", Plural: "notes", Fields: []domain.Field{
			{Name: "title", Type: domain.FieldString},
			{Name: "body", Type: domain.FieldString, Optional: true},
			{Name: "summary", Type: domain.FieldString, Optional: true, Min: bound(10), Max: bound(200)},
			{Name: "slug", Type: domain.FieldString, Min: bound(3)},
			{Name: "tagline", Type: domain.FieldString, Optional: true, Max: bound(50)},
			{Name: "contact", Type: domain.FieldEmail},
			{Name: "backup", Type: domain.FieldEmail, Optional: true},
			{Name: "owner_id", Type: domain.FieldRef, Ref: "owner"},
		},
	}}})
	props := at(t, doc, "components", "schemas", "NoteInput", "properties").(map[string]any)
	field := func(name string) map[string]any { return props[name].(map[string]any) }

	// Validate rejects a required string that is blank or only spaces
	for _, name := range []string{"title", "slug", "owner_id"} {
		if field(name)["pattern"] != `\S` {
			t.Errorf("%s = %v, want a pattern requiring a non-space character", name, field(name))
		}
	}
	if _, has := field("body")["pattern"]; has {
		t.Errorf("body = %v: an optional string may be blank", field("body"))
	}
	if _, has := field("contact")["pattern"]; has {
		t.Errorf("contact = %v: an email's format already rules out blank", field("contact"))
	}

	// Validate skips an optional field's rules when it is empty, so "" must stay valid
	for _, name := range []string{"summary", "backup"} {
		anyOf, ok := field(name)["anyOf"].([]any)
		if !ok || len(anyOf) != 2 || anyOf[0].(map[string]any)["pattern"] != `^\s*$` {
			t.Errorf("%s = %v, want anyOf an empty value or the rules", name, field(name))
		}
		if field(name)["type"] != "string" {
			t.Errorf("%s = %v, want type string kept for client generators", name, field(name))
		}
	}
	rules := field("summary")["anyOf"].([]any)[1].(map[string]any)
	if rules["minLength"] != 10.0 || rules["maxLength"] != 200.0 {
		t.Errorf("summary's rules = %v, want 10 to 200", rules)
	}
	// An optional string with only a maximum has nothing an empty value breaks
	if f := field("tagline"); f["maxLength"] != 50.0 || f["anyOf"] != nil {
		t.Errorf("tagline = %v, want a plain maxLength", f)
	}
	if field("slug")["minLength"] != 3.0 {
		t.Errorf("slug = %v, want minLength 3 beside the non-blank pattern", field("slug"))
	}
}
