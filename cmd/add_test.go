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

	for _, want := range []string{"products/product.go", "products/handler_test.go", "server/routes.go", "POST   /products"} {
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
