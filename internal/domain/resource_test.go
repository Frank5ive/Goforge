package domain

import (
	"strings"
	"testing"
)

func validResource() ResourceConfig {
	return ResourceConfig{
		ProjectDir: ".",
		Name:       "product",
		Fields:     []Field{{Name: "title", Type: FieldString}},
	}
}

func TestResourceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *ResourceConfig)
		wantErr string
	}{
		{"valid", func(c *ResourceConfig) {}, ""},
		{"valid snake_case", func(c *ResourceConfig) { c.Name = "blog_post" }, ""},
		{"valid explicit plural", func(c *ResourceConfig) { c.Name, c.Plural = "person", "people" }, ""},
		{"all field types", func(c *ResourceConfig) {
			c.Fields = []Field{{"a", FieldString, false}, {"b", FieldInt, false}, {"c", FieldFloat, false}, {"d", FieldBool, false}, {"e", FieldTime, false}}
		}, ""},
		{"optional string", func(c *ResourceConfig) { c.Fields = []Field{{"nickname", FieldString, true}} }, ""},
		{"optional non-string", func(c *ResourceConfig) { c.Fields = []Field{{"age", FieldInt, true}} }, "only string fields"},
		{"empty project dir", func(c *ResourceConfig) { c.ProjectDir = "" }, "project directory"},
		{"uppercase name", func(c *ResourceConfig) { c.Name = "Product" }, "snake_case"},
		{"hyphenated name", func(c *ResourceConfig) { c.Name = "blog-post" }, "snake_case"},
		{"double underscore", func(c *ResourceConfig) { c.Name = "blog__post" }, "snake_case"},
		{"keyword name", func(c *ResourceConfig) { c.Name = "type" }, "reserved"},
		{"colliding type name", func(c *ResourceConfig) { c.Name = "service" }, "reserved"},
		{"reserved package", func(c *ResourceConfig) { c.Name, c.Plural = "srv", "server" }, "reserved"},
		{"plural equals name", func(c *ResourceConfig) { c.Name, c.Plural = "sheep", "sheep" }, "must differ"},
		{"no fields", func(c *ResourceConfig) { c.Fields = nil }, "at least one field"},
		{"automatic field", func(c *ResourceConfig) { c.Fields = []Field{{"id", FieldString, false}} }, "added automatically"},
		{"unsupported type", func(c *ResourceConfig) { c.Fields = []Field{{"price", "decimal", false}} }, "unsupported type"},
		{"duplicate field", func(c *ResourceConfig) {
			c.Fields = []Field{{"title", FieldString, false}, {"title", FieldInt, false}}
		}, "more than once"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validResource()
			tt.modify(&c)

			err := c.Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() error = %v, want nil", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("Validate() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestResourceConfig_Names(t *testing.T) {
	c := ResourceConfig{Name: "blog_post"}

	if got := c.PluralName(); got != "blog_posts" {
		t.Errorf("PluralName() = %q, want blog_posts", got)
	}
	if got := c.PackageName(); got != "blogposts" {
		t.Errorf("PackageName() = %q, want blogposts", got)
	}
	if got := c.RoutePath(); got != "/blog-posts" {
		t.Errorf("RoutePath() = %q, want /blog-posts", got)
	}
}

func TestParseFields(t *testing.T) {
	fields, err := ParseFields(" title:string, price:FLOAT ,in-stock:bool, notes , image_url:string?")
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}

	want := []Field{
		{"title", FieldString, false}, {"price", FieldFloat, false}, {"in_stock", FieldBool, false},
		{"notes", FieldString, false}, {"image_url", FieldString, true},
	}
	if len(fields) != len(want) {
		t.Fatalf("ParseFields() = %v, want %v", fields, want)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Errorf("field %d = %v, want %v", i, fields[i], want[i])
		}
	}

	if _, err := ParseFields(":int"); err == nil {
		t.Error("ParseFields(\":int\") error = nil, want missing name error")
	}
}

func TestPluralize(t *testing.T) {
	for word, want := range map[string]string{
		"product":   "products",
		"category":  "categories",
		"day":       "days",
		"box":       "boxes",
		"status":    "statuses",
		"match":     "matches",
		"blog_post": "blog_posts",
	} {
		if got := Pluralize(word); got != want {
			t.Errorf("Pluralize(%q) = %q, want %q", word, got, want)
		}
	}
}

func TestCamelCase(t *testing.T) {
	for snake, want := range map[string]string{
		"title":     "Title",
		"in_stock":  "InStock",
		"image_url": "ImageURL",
		"user_id":   "UserID",
		"blog_post": "BlogPost",
	} {
		if got := CamelCase(snake); got != want {
			t.Errorf("CamelCase(%q) = %q, want %q", snake, got, want)
		}
	}
}
