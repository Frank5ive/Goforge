package usecase

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Frank5ive/goforge/internal/domain"
)

// mockModules records Require calls and can be told to fail.
type mockModules struct {
	calls []string
	err   error
}

func (m *mockModules) Require(dir, module, version string) error {
	m.calls = append(m.calls, module+"@"+version)
	return m.err
}

// Minimal versions of the generated files `add db` edits, with their markers.
const (
	testConfigGo = `package config

type Config struct {
	Port int

	// goforge:config-fields (marker)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{}
	var errs []error
	// goforge:config-load (marker)

	return cfg, nil
}
`
	testDepsGo = `package server

import "context"

type Deps struct {
	// goforge:deps-fields (marker)
}

func (d Deps) Ready(ctx context.Context) error {
	// goforge:deps-ready (marker)
	return ctx.Err()
}
`
	testMainGo = `package main

import (
	"context"

	"example.com/app/server"
)

func run(ctx context.Context) error {
	var deps server.Deps
	// goforge:deps-connect (marker)

	_ = deps
	return nil
}
`
	testManifest = `{"app_name": "my-app", "module": "example.com/app", "port": 3000, "features": {"docker": true},
  "resources": [{"name": "product", "plural": "products", "fields": [{"name": "title", "type": "string"}]}]}`
)

// newDBProject returns a mock filesystem holding a generated project at /proj
// with one in-memory resource.
func newDBProject() *MockFileWriter {
	w := newProjectWriter()
	routes, _ := registerResource(testRoutes, "example.com/app", "products", "products.NewMemoryRepository()", nil)
	files := map[string]string{
		"server/routes.go":    routes,
		"config/config.go":    testConfigGo,
		"server/deps.go":      testDepsGo,
		"main.go":             testMainGo,
		".goforge.json":       testManifest,
		"Makefile":            "run: ## Run the server\n\tgo run .\n",
		"products/product.go": "package products\n",
	}
	for rel, content := range files {
		w.FilesWritten[filepath.Join("/proj", filepath.FromSlash(rel))] = content
	}
	return w
}

func newTestDatabaseGenerator(w *MockFileWriter, modules *mockModules) *DatabaseGeneratorUsecase {
	u := NewDatabaseGeneratorUsecase(NewMockTemplateRenderer(), w, modules)
	u.now = func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }
	return u
}

func file(w *MockFileWriter, rel string) string {
	return w.FilesWritten[filepath.Join("/proj", filepath.FromSlash(rel))]
}

func TestDatabaseGenerator_Add(t *testing.T) {
	w := newDBProject()
	modules := &mockModules{}

	changes, err := newTestDatabaseGenerator(w, modules).Add(&domain.DatabaseConfig{ProjectDir: "/proj", Engine: "postgres"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	for _, rel := range []string{
		"db/db.go", "db/migrate.go", "db/migrations/README.md", "db/dbtest/dbtest.go", "docker-compose.yml",
		"products/postgres.go", "products/postgres_test.go", "products/repository_test.go",
		"db/migrations/20260301120000_create_products.sql",
	} {
		if _, ok := w.FilesWritten[filepath.Join("/proj", filepath.FromSlash(rel))]; !ok {
			t.Errorf("expected %s to be created", rel)
		}
	}

	checks := map[string][]string{
		"config/config.go": {"DatabaseURL string", `getenv("DATABASE_URL")`, "postgres://app:app@localhost:5432/my_app?sslmode=disable"},
		"server/deps.go":   {`"github.com/jackc/pgx/v5/pgxpool"`, "DB *pgxpool.Pool", "d.DB.Ping(ctx)"},
		"main.go":          {`"example.com/app/db"`, "db.Connect(ctx, cfg.DatabaseURL, log)", "defer db.Close(database, log)", "deps.DB = database"},
		"server/routes.go": {"products.NewPostgresRepository(deps.DB)"},
		"Makefile":         {"db-up:", "test-db: db-up", "my_app_test"},
		".goforge.json":    {`"database": "postgres"`, `"name": "product"`},
	}
	for rel, wants := range checks {
		content := file(w, rel)
		for _, want := range wants {
			if !strings.Contains(content, want) {
				t.Errorf("%s does not contain %q:\n%s", rel, want, content)
			}
		}
	}
	if strings.Contains(file(w, "server/routes.go"), "NewMemoryRepository") {
		t.Error("routes.go still uses the in-memory repository")
	}

	if len(modules.calls) != 1 || modules.calls[0] != postgresEngine.module+"@"+postgresEngine.version {
		t.Errorf("module calls = %v, want pgx", modules.calls)
	}
	if len(changes.Created) == 0 || len(changes.Updated) == 0 {
		t.Errorf("changes = %+v, want created and updated files", changes)
	}
}

func TestDatabaseGenerator_Add_RefusesAndChangesNothing(t *testing.T) {
	tests := []struct {
		name  string
		setup func(w *MockFileWriter)
		want  string
	}{
		{
			name: "database already added",
			setup: func(w *MockFileWriter) {
				w.FilesWritten[filepath.Join("/proj", ".goforge.json")] = `{"module": "example.com/app", "database": "postgres"}`
			},
			want: "already uses a postgres database",
		},
		{
			name: "marker missing",
			setup: func(w *MockFileWriter) {
				w.FilesWritten[filepath.Join("/proj", "main.go")] = strings.ReplaceAll(testMainGo, "goforge:deps-connect", "removed")
			},
			want: "goforge:deps-connect",
		},
		{
			name: "older project with resources but no manifest",
			setup: func(w *MockFileWriter) {
				delete(w.FilesWritten, filepath.Join("/proj", ".goforge.json"))
			},
			want: "made by an older GoForge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newDBProject()
			tt.setup(w)
			before := len(w.FilesWritten)
			modules := &mockModules{}

			_, err := newTestDatabaseGenerator(w, modules).Add(&domain.DatabaseConfig{ProjectDir: "/proj", Engine: "postgres"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Add() error = %v, want it to contain %q", err, tt.want)
			}
			if len(w.FilesWritten) != before || len(modules.calls) != 0 {
				t.Errorf("a refused run must not write files or change go.mod")
			}
		})
	}
}

func TestDatabaseGenerator_Add_RollsBackWhenDependencyFails(t *testing.T) {
	w := newDBProject()
	w.RemoveAllFunc = func(path string) error {
		delete(w.FilesWritten, path)
		return nil
	}
	original := map[string]string{}
	for path, content := range w.FilesWritten {
		original[path] = content
	}

	_, err := newTestDatabaseGenerator(w, &mockModules{err: errors.New("network down")}).
		Add(&domain.DatabaseConfig{ProjectDir: "/proj", Engine: "postgres"})
	if err == nil || !strings.Contains(err.Error(), "left unchanged") {
		t.Fatalf("Add() error = %v, want a rollback error", err)
	}

	if len(w.FilesWritten) != len(original) {
		t.Errorf("rollback left %d files, want the original %d", len(w.FilesWritten), len(original))
	}
	for path, content := range original {
		if w.FilesWritten[path] != content {
			t.Errorf("%s was not restored", path)
		}
	}
}

func TestDatabaseGenerator_Add_UnsupportedEngine(t *testing.T) {
	_, err := newTestDatabaseGenerator(newDBProject(), &mockModules{}).
		Add(&domain.DatabaseConfig{ProjectDir: "/proj", Engine: "oracle"})
	if err == nil || !strings.Contains(err.Error(), "unsupported database") {
		t.Errorf("Add() error = %v, want unsupported database", err)
	}
}

func TestDatabaseName(t *testing.T) {
	for _, tt := range []struct{ app, module, want string }{
		{"my-api", "example.com/my-api", "my_api"},
		{"", "github.com/acme/Shop-API", "shop_api"},
		{"shop", "shop", "shop"},
	} {
		if got := databaseName(tt.app, tt.module); got != tt.want {
			t.Errorf("databaseName(%q, %q) = %q, want %q", tt.app, tt.module, got, tt.want)
		}
	}
}

func TestDatabaseGenerator_Add_ConvertsTheRightResourceWhenNamesOverlap(t *testing.T) {
	w := newDBProject()
	// "posts" is a suffix of "blogposts", and the blogposts registration comes first
	// (e.g. the user reordered routes.go)
	w.FilesWritten[filepath.Join("/proj", ".goforge.json")] = `{"module": "example.com/app", "resources": [
		{"name": "post", "plural": "posts", "fields": [{"name": "title", "type": "string"}]},
		{"name": "blog_post", "plural": "blog_posts", "fields": [{"name": "title", "type": "string"}]}]}`
	routes, _ := registerResource(testRoutes, "example.com/app", "posts", "posts.NewMemoryRepository()", nil)
	routes, _ = registerResource(routes, "example.com/app", "blogposts", "blogposts.NewMemoryRepository()", nil)
	// Move the blogposts registration above posts
	routes = strings.Replace(routes,
		"\tposts.RegisterRoutes(mux, posts.NewService(posts.NewMemoryRepository()))\n\tblogposts.RegisterRoutes(mux, blogposts.NewService(blogposts.NewMemoryRepository()))\n",
		"\tblogposts.RegisterRoutes(mux, blogposts.NewService(blogposts.NewMemoryRepository()))\n\tposts.RegisterRoutes(mux, posts.NewService(posts.NewMemoryRepository()))\n", 1)
	w.FilesWritten[filepath.Join("/proj", "server", "routes.go")] = routes

	if _, err := newTestDatabaseGenerator(w, &mockModules{}).Add(&domain.DatabaseConfig{ProjectDir: "/proj", Engine: "postgres"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got := file(w, "server/routes.go")
	for _, want := range []string{
		"posts.RegisterRoutes(mux, posts.NewService(posts.NewPostgresRepository(deps.DB)))",
		"blogposts.RegisterRoutes(mux, blogposts.NewService(blogposts.NewPostgresRepository(deps.DB)))",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("routes.go does not contain %q:\n%s", want, got)
		}
	}
}

func TestCIServices(t *testing.T) {
	tests := []struct {
		engine *engine
		want   string
	}{
		// MongoDB refuses to start on Linux 6.19 through 7.0.13 unless glibc owns rseq
		{mongoEngine, "GLIBC_TUNABLES: glibc.pthread.rseq=1"},
		// Postgres runs a socket-only server while initializing, so probe over TCP
		{postgresEngine, `pg_isready -U app -d shop_test -h 127.0.0.1"`},
		{mysqlEngine, "mysqladmin ping -h 127.0.0.1"},
	}
	for _, tt := range tests {
		if got := tt.engine.ciService("shop_test", tt.engine.testURL("shop")); !strings.Contains(got, tt.want) {
			t.Errorf("%s CI service does not contain %q:\n%s", tt.engine.name, tt.want, got)
		}
	}
}
