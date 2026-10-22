package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runAddResourceCmd runs `add resource` with args against the real templates.
func runAddResourceCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))

	cmd := newAddCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"resource"}, args...))

	err := cmd.Execute()
	return out.String(), err
}

// generatedProject creates a project with `generate` and returns its directory.
func generatedProject(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if output, err := runGenerateCmd(t, "--name", "shop", "--output", dir, "--yes"); err != nil {
		t.Fatalf("generate failed: %v\n%s", err, output)
	}
	return filepath.Join(dir, "shop")
}

func TestAddResource(t *testing.T) {
	project := generatedProject(t)

	output, err := runAddResourceCmd(t, "product", "--dir", project, "--fields", "name:string,price:float")
	if err != nil {
		t.Fatalf("add resource failed: %v\n%s", err, output)
	}

	for _, want := range []string{"products/product.go", "products/handler_test.go", "products/requests.http", "server/routes.go", "POST   /products", "PATCH  /products/{id}"} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not mention %q:\n%s", want, output)
		}
	}

	routes, err := os.ReadFile(filepath.Join(project, "server", "routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routes), "products.RegisterRoutes") {
		t.Errorf("routes.go does not register products:\n%s", routes)
	}
}

func TestAddResource_DryRunWritesNothing(t *testing.T) {
	project := generatedProject(t)
	routesBefore, _ := os.ReadFile(filepath.Join(project, "server", "routes.go"))

	output, err := runAddResourceCmd(t, "product", "--dir", project, "--dry-run")
	if err != nil {
		t.Fatalf("add resource failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(project, "products")); !os.IsNotExist(err) {
		t.Error("dry run must not create the resource package")
	}
	if routesAfter, _ := os.ReadFile(filepath.Join(project, "server", "routes.go")); !bytes.Equal(routesBefore, routesAfter) {
		t.Error("dry run must not change routes.go")
	}
	if !strings.Contains(output, "Dry run") || !strings.Contains(output, "would update") {
		t.Errorf("unexpected dry run output:\n%s", output)
	}
}

func TestAddResource_Errors(t *testing.T) {
	project := generatedProject(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"outside a project", []string{"product", "--dir", t.TempDir()}, "not a Go module"},
		{"bad field type", []string{"product", "--dir", project, "--fields", "price:money"}, "unsupported type"},
		{"bad name", []string{"Product", "--dir", project}, "snake_case"},
		{"missing name", []string{"--dir", project}, "accepts 1 arg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runAddResourceCmd(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestAddResource_AcceptsHyphenatedName(t *testing.T) {
	project := generatedProject(t)

	if output, err := runAddResourceCmd(t, "blog-post", "--dir", project); err != nil {
		t.Fatalf("add resource failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(project, "blogposts", "blog_post.go")); err != nil {
		t.Errorf("expected blogposts/blog_post.go: %v", err)
	}
}

func TestAddResource_RequestsFileUsesProjectPort(t *testing.T) {
	dir := t.TempDir()
	if output, err := runGenerateCmd(t, "--name", "shop", "--port", "3000", "--output", dir, "--yes"); err != nil {
		t.Fatalf("generate failed: %v\n%s", err, output)
	}
	project := filepath.Join(dir, "shop")

	if output, err := runAddResourceCmd(t, "product", "--dir", project); err != nil {
		t.Fatalf("add resource failed: %v\n%s", err, output)
	}

	requests, err := os.ReadFile(filepath.Join(project, "products", "requests.http"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@baseUrl = http://localhost:3000", "PATCH {{baseUrl}}/products/{{id}}", `"name": "example name"`} {
		if !strings.Contains(string(requests), want) {
			t.Errorf("requests.http does not contain %q:\n%s", want, requests)
		}
	}
}

// runAddDBCmd runs `add db` with args against the real templates.
func runAddDBCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))

	cmd := newAddCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"db"}, args...))

	err := cmd.Execute()
	return out.String(), err
}

func TestAddDB_DryRunWritesNothing(t *testing.T) {
	project := generatedProject(t)
	if output, err := runAddResourceCmd(t, "product", "--dir", project); err != nil {
		t.Fatalf("add resource failed: %v\n%s", err, output)
	}
	goModBefore, _ := os.ReadFile(filepath.Join(project, "go.mod"))

	output, err := runAddDBCmd(t, "postgres", "--dir", project, "--dry-run")
	if err != nil {
		t.Fatalf("add db --dry-run failed: %v\n%s", err, output)
	}

	for _, want := range []string{"Dry run", "db/db.go", "products/postgres.go", "docker-compose.yml", "server/routes.go"} {
		if !strings.Contains(output, want) {
			t.Errorf("dry run output does not mention %q:\n%s", want, output)
		}
	}
	if _, err := os.Stat(filepath.Join(project, "db")); !os.IsNotExist(err) {
		t.Error("dry run must not create the db package")
	}
	if goModAfter, _ := os.ReadFile(filepath.Join(project, "go.mod")); !bytes.Equal(goModBefore, goModAfter) {
		t.Error("dry run must not change go.mod")
	}
}

func TestAddDB_Errors(t *testing.T) {
	project := generatedProject(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unsupported engine", []string{"mysql", "--dir", project}, "unsupported database"},
		{"outside a project", []string{"postgres", "--dir", t.TempDir()}, "not a Go module"},
		{"missing engine", []string{"--dir", project}, "accepts 1 arg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runAddDBCmd(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}
