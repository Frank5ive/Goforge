package domain

import (
	"reflect"
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
			c.Fields = []Field{{Name: "a", Type: FieldString, Optional: false}, {Name: "b", Type: FieldInt, Optional: false}, {Name: "c", Type: FieldFloat, Optional: false}, {Name: "d", Type: FieldBool, Optional: false}, {Name: "e", Type: FieldTime, Optional: false}}
		}, ""},
		{"optional string", func(c *ResourceConfig) { c.Fields = []Field{{Name: "nickname", Type: FieldString, Optional: true}} }, ""},
		{"optional non-string", func(c *ResourceConfig) { c.Fields = []Field{{Name: "age", Type: FieldInt, Optional: true}} }, "only string and email fields"},
		{"empty project dir", func(c *ResourceConfig) { c.ProjectDir = "" }, "project directory"},
		{"uppercase name", func(c *ResourceConfig) { c.Name = "Product" }, "snake_case"},
		{"hyphenated name", func(c *ResourceConfig) { c.Name = "blog-post" }, "snake_case"},
		{"double underscore", func(c *ResourceConfig) { c.Name = "blog__post" }, "snake_case"},
		{"keyword name", func(c *ResourceConfig) { c.Name = "type" }, "reserved"},
		{"colliding type name", func(c *ResourceConfig) { c.Name = "service" }, "reserved"},
		{"reserved package", func(c *ResourceConfig) { c.Name, c.Plural = "srv", "server" }, "reserved"},
		{"plural equals name", func(c *ResourceConfig) { c.Name, c.Plural = "sheep", "sheep" }, "must differ"},
		{"no fields", func(c *ResourceConfig) { c.Fields = nil }, "at least one field"},
		{"automatic field", func(c *ResourceConfig) { c.Fields = []Field{{Name: "id", Type: FieldString, Optional: false}} }, "added automatically"},
		{"unsupported type", func(c *ResourceConfig) { c.Fields = []Field{{Name: "price", Type: "decimal", Optional: false}} }, "unsupported type"},
		{"duplicate field", func(c *ResourceConfig) {
			c.Fields = []Field{{Name: "title", Type: FieldString, Optional: false}, {Name: "title", Type: FieldInt, Optional: false}}
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
		{Name: "title", Type: FieldString, Optional: false}, {Name: "price", Type: FieldFloat, Optional: false}, {Name: "in_stock", Type: FieldBool, Optional: false},
		{Name: "notes", Type: FieldString, Optional: false}, {Name: "image_url", Type: FieldString, Optional: true},
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

func TestParseFields_Rules(t *testing.T) {
	bound := func(v float64) *float64 { return &v }
	fields, err := ParseFields("name:string(1..100), price:float(0..), qty:int(..9), contact:email, alt:email?, nick:string?(3..), code:STRING(2..2)?")
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}

	want := []Field{
		{Name: "name", Type: FieldString, Min: bound(1), Max: bound(100)},
		{Name: "price", Type: FieldFloat, Min: bound(0)},
		{Name: "qty", Type: FieldInt, Max: bound(9)},
		{Name: "contact", Type: FieldEmail},
		{Name: "alt", Type: FieldEmail, Optional: true},
		{Name: "nick", Type: FieldString, Optional: true, Min: bound(3)},
		{Name: "code", Type: FieldString, Optional: true, Min: bound(2), Max: bound(2)},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("ParseFields() = %+v, want %+v", fields, want)
	}
	for _, f := range fields {
		if err := validateFields([]Field{f}); err != nil {
			t.Errorf("%s: %v", f.Spec(), err)
		}
		// Spec writes a field back in the syntax it was parsed from
		again, err := ParseFields(f.Spec())
		if err != nil || !reflect.DeepEqual(again[0], f) {
			t.Errorf("ParseFields(%q) = %+v, %v; want %+v", f.Spec(), again, err, f)
		}
	}
}

func TestParseFields_InvalidRanges(t *testing.T) {
	for _, spec := range []string{"n:int(1-5)", "n:int(..)", "n:int(a..b)", "n:int(1..5", "n:float(1..inf)"} {
		if _, err := ParseFields(spec); err == nil {
			t.Errorf("ParseFields(%q) = nil error, want one", spec)
		}
	}
}

func TestValidateBounds(t *testing.T) {
	bound := func(v float64) *float64 { return &v }
	tests := []struct {
		field Field
		want  string
	}{
		{Field{Name: "ok", Type: FieldBool, Min: bound(1)}, "applies to string, int, and float"},
		{Field{Name: "e", Type: FieldEmail, Max: bound(5)}, "applies to string, int, and float"},
		{Field{Name: "n", Type: FieldInt, Min: bound(1.5)}, "whole numbers"},
		{Field{Name: "s", Type: FieldString, Min: bound(-1)}, "can't be negative"},
		{Field{Name: "f", Type: FieldFloat, Min: bound(5), Max: bound(1)}, "minimum is above its maximum"},
	}
	for _, tt := range tests {
		if err := validateFields([]Field{tt.field}); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want it to contain %q", tt.field.Spec(), err, tt.want)
		}
	}
	if err := validateFields([]Field{{Name: "price", Type: FieldFloat, Min: bound(0.5), Max: bound(9.99)}}); err != nil {
		t.Errorf("float bounds may be fractional: %v", err)
	}
}
