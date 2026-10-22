package usecase

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

const testRoutes = `package server

import (
	"net/http"

	"example.com/app/handlers"
)

func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", handlers.Health("app"))

	// goforge:routes (` + "`goforge add resource`" + ` registers new resources above this line)
}
`

// newProjectWriter returns a mock filesystem containing a generated project's
// go.mod and routes.go at /proj.
func newProjectWriter() *MockFileWriter {
	writer := NewMockFileWriter()
	writer.FilesWritten[filepath.Join("/proj", "go.mod")] = "module example.com/app\n\ngo 1.24\n"
	writer.FilesWritten[filepath.Join("/proj", "server", "routes.go")] = testRoutes
	return writer
}

func testResource() *domain.ResourceConfig {
	return &domain.ResourceConfig{
		ProjectDir: "/proj",
		Name:       "blog_post",
		Fields:     []domain.Field{{Name: "title", Type: domain.FieldString}},
	}
}

func TestResourceGenerator_Add(t *testing.T) {
	renderer := NewMockTemplateRenderer()
	writer := newProjectWriter()
	usecase := NewResourceGeneratorUsecase(renderer, writer)

	if err := usecase.Add(testResource()); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	pkgDir := filepath.Join("/proj", "blogposts")
	for _, name := range []string{"blog_post.go", "memory.go", "service.go", "handler.go", "service_test.go", "handler_test.go", "requests.http"} {
		if _, ok := writer.FilesWritten[filepath.Join(pkgDir, name)]; !ok {
			t.Errorf("Expected %s to be written", name)
		}
	}

	routes := writer.FilesWritten[filepath.Join("/proj", "server", "routes.go")]
	for _, want := range []string{
		`"example.com/app/blogposts"`,
		"blogposts.RegisterRoutes(mux, blogposts.NewService(blogposts.NewMemoryRepository()))",
	} {
		if !strings.Contains(routes, want) {
			t.Errorf("routes.go does not contain %q:\n%s", want, routes)
		}
	}
}

func TestResourceGenerator_Add_Failures(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(w *MockFileWriter, r *MockTemplateRenderer)
		step     string
		cleansUp bool
	}{
		{
			name: "not a go module",
			setup: func(w *MockFileWriter, _ *MockTemplateRenderer) {
				delete(w.FilesWritten, filepath.Join("/proj", "go.mod"))
			},
			step: "project_check",
		},
		{
			name: "missing routes marker",
			setup: func(w *MockFileWriter, _ *MockTemplateRenderer) {
				w.FilesWritten[filepath.Join("/proj", "server", "routes.go")] = strings.ReplaceAll(testRoutes, routesMarker, "// routes")
			},
			step: "register_routes",
		},
		{
			name:  "package already exists",
			setup: func(w *MockFileWriter, _ *MockTemplateRenderer) { w.FileExistsFunc = func(string) bool { return true } },
			step:  "directory_check",
		},
		{
			name: "template fails",
			setup: func(_ *MockFileWriter, r *MockTemplateRenderer) {
				r.RenderFunc = func(name string, _ interface{}) (string, error) {
					if name == "resource/service.tmpl" {
						return "", fmt.Errorf("boom")
					}
					return "package x\n", nil
				}
			},
			step:     "generate_files",
			cleansUp: true,
		},
		{
			name: "routes write fails",
			setup: func(w *MockFileWriter, _ *MockTemplateRenderer) {
				w.WriteFileFunc = func(path string, content string) error {
					if strings.HasSuffix(path, "routes.go") {
						return fmt.Errorf("read-only")
					}
					w.FilesWritten[path] = content
					return nil
				}
			},
			step:     "register_routes",
			cleansUp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewMockTemplateRenderer()
			writer := newProjectWriter()
			tt.setup(writer, renderer)

			err := NewResourceGeneratorUsecase(renderer, writer).Add(testResource())
			if err == nil {
				t.Fatal("Add() error = nil, want an error")
			}
			assertGenerationStep(t, err, tt.step)

			removed := len(writer.PathsRemoved) == 1 && writer.PathsRemoved[0] == filepath.Join("/proj", "blogposts")
			if removed != tt.cleansUp {
				t.Errorf("package removed = %t, want %t (removed: %v)", removed, tt.cleansUp, writer.PathsRemoved)
			}
		})
	}
}

func TestResourceGenerator_Add_WritesNothingWhenInvalid(t *testing.T) {
	writer := newProjectWriter()
	config := testResource()
	config.Name = "Bad-Name"

	err := NewResourceGeneratorUsecase(NewMockTemplateRenderer(), writer).Add(config)
	assertGenerationStep(t, err, "validation")

	if len(writer.DirsCreated) != 0 {
		t.Errorf("Expected nothing to be created, got %v", writer.DirsCreated)
	}
}

func TestRegisterResource(t *testing.T) {
	routes, err := registerResource(testRoutes, "example.com/app", "products")
	if err != nil {
		t.Fatalf("registerResource() error = %v", err)
	}

	// The import joins the module group and is sorted after handlers
	handlersAt := strings.Index(routes, `"example.com/app/handlers"`)
	productsAt := strings.Index(routes, `"example.com/app/products"`)
	if handlersAt < 0 || productsAt < handlersAt {
		t.Errorf("expected products import after handlers import:\n%s", routes)
	}

	// The registration goes just above the marker, so later resources stack up
	registrationAt := strings.Index(routes, "products.RegisterRoutes(mux")
	if registrationAt < 0 || registrationAt > strings.Index(routes, routesMarker) {
		t.Errorf("expected registration above the marker:\n%s", routes)
	}

	if _, err := registerResource(routes, "example.com/app", "products"); err == nil {
		t.Error("registering the same resource twice should fail")
	}

	again, err := registerResource(routes, "example.com/app", "categories")
	if err != nil {
		t.Fatalf("registering a second resource: %v", err)
	}
	if !strings.Contains(again, "products.RegisterRoutes") || !strings.Contains(again, "categories.RegisterRoutes") {
		t.Errorf("expected both resources to be registered:\n%s", again)
	}
}

func TestNewResourceData(t *testing.T) {
	data := newResourceData(&domain.ResourceConfig{
		Name: "blog_post",
		Fields: []domain.Field{
			{Name: "title", Type: domain.FieldString},
			{Name: "image_url", Type: domain.FieldString, Optional: true},
			{Name: "published_at", Type: domain.FieldTime},
		},
	}, "example.com/app")

	if data.Package != "blogposts" || data.Type != "BlogPost" || data.Path != "/blog-posts" || data.Human != "blog post" {
		t.Errorf("unexpected names: %+v", data)
	}
	if data.RequiredField != "Title" {
		t.Errorf("RequiredField = %q, want Title", data.RequiredField)
	}
	if data.First.GoName != "Title" {
		t.Errorf("First = %+v, want the title field", data.First)
	}
	if want := "{\n  \"title\": \"example title\",\n  \"image_url\": \"example image url\",\n  \"published_at\": \"2026-01-02T15:04:05Z\"\n}"; data.SampleBody != want {
		t.Errorf("SampleBody = %q, want %q", data.SampleBody, want)
	}
	if !data.Fields[0].Required || data.Fields[1].Required {
		t.Errorf("Required = %t, %t; want title required and image_url optional", data.Fields[0].Required, data.Fields[1].Required)
	}
	if f := data.Fields[1]; f.GoName != "ImageURL" || f.JSON != "image_url" {
		t.Errorf("image_url field = %+v", f)
	}
	if f := data.Fields[2]; f.GoType != "time.Time" || !strings.HasPrefix(f.SampleJSON, `"2026-`) {
		t.Errorf("time field = %+v", f)
	}
}

func TestResourceGenerator_ReadDefaultPort(t *testing.T) {
	writer := newProjectWriter()
	usecase := NewResourceGeneratorUsecase(NewMockTemplateRenderer(), writer)

	if got := usecase.readDefaultPort("/proj"); got != fallbackPort {
		t.Errorf("without config.go: port = %d, want %d", got, fallbackPort)
	}

	writer.FilesWritten[filepath.Join("/proj", "config", "config.go")] = "const (\n\tdefaultPort            = 3000\n)\n"
	if got := usecase.readDefaultPort("/proj"); got != 3000 {
		t.Errorf("with config.go: port = %d, want 3000", got)
	}
}
