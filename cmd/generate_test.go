package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// runGenerateCmd runs the generate command with args against the real templates
// and returns its output. Templates are read from disk so tests don't depend on main.
func runGenerateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))

	cmd := newGenerateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func TestGenerate_WithFlags(t *testing.T) {
	dir := t.TempDir()

	output, err := runGenerateCmd(t, "--name", "flag-api", "--module", "github.com/acme/flag-api",
		"--output", dir, "--docker=false", "--yes")
	if err != nil {
		t.Fatalf("generate failed: %v\n%s", err, output)
	}

	project := filepath.Join(dir, "flag-api")

	goMod, err := os.ReadFile(filepath.Join(project, "go.mod"))
	if err != nil {
		t.Fatalf("Expected go.mod: %v", err)
	}
	if !strings.HasPrefix(string(goMod), "module github.com/acme/flag-api\n") {
		t.Errorf("Expected module path from --module, got:\n%s", goMod)
	}

	if _, err := os.Stat(filepath.Join(project, "Dockerfile")); !os.IsNotExist(err) {
		t.Error("Expected no Dockerfile with --docker=false")
	}
	if _, err := os.Stat(filepath.Join(project, "logger", "logger.go")); err != nil {
		t.Error("Expected logger by default")
	}

	if !strings.Contains(output, "Created") || !strings.Contains(output, "server/server.go") {
		t.Errorf("Expected created files to be listed, got:\n%s", output)
	}
}

func TestGenerate_DryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()

	output, err := runGenerateCmd(t, "--name", "dry-api", "--output", dir, "--dry-run", "--yes")
	if err != nil {
		t.Fatalf("generate failed: %v\n%s", err, output)
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("Dry run must not write anything, found %d entries", len(entries))
	}

	for _, want := range []string{"Dry run", "main.go", "Dockerfile"} {
		if !strings.Contains(output, want) {
			t.Errorf("Expected dry run output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestGenerate_InitGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()

	output, err := runGenerateCmd(t, "--name", "git-api", "--output", dir, "--git", "--yes")
	if err != nil {
		t.Fatalf("generate failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(dir, "git-api", ".git")); err != nil {
		t.Errorf("Expected a git repository: %v\n%s", err, output)
	}
}

func TestGenerate_WithoutTerminalUsesDefaults(t *testing.T) {
	// go test's stdin is not a terminal, so generate must not prompt (or hang):
	// like --yes, it uses the flag values and defaults
	dir := t.TempDir()
	if output, err := runGenerateCmd(t, "--name", "no-tty-api", "--output", dir); err != nil {
		t.Fatalf("generate without a terminal failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(dir, "no-tty-api", "Dockerfile")); err != nil {
		t.Errorf("expected the default features to be generated: %v", err)
	}
}

func TestGenerate_NoPromptWhenAllOptionsSetByFlag(t *testing.T) {
	dir := t.TempDir()

	output, err := runGenerateCmd(t, "--name", "all-flags", "--output", dir,
		"--module", "example.com/all-flags", "--logger=false", "--docker=false", "--readme=false", "--ci=false", "--db=")
	if err != nil {
		t.Fatalf("Expected no prompt when every option is set by flag, got: %v\n%s", err, output)
	}
}

func TestGenerate_RejectsExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := runGenerateCmd(t, "--name", "taken", "--output", dir, "--yes")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got %v", err)
	}
}

func TestGenerate_RejectsInvalidModule(t *testing.T) {
	_, err := runGenerateCmd(t, "--name", "bad-module", "--output", t.TempDir(), "--module", "not a path", "--yes")
	if err == nil || !strings.Contains(err.Error(), "invalid module path") {
		t.Errorf("Expected invalid module path error, got %v", err)
	}
}

func TestGenerate_WithPostgres(t *testing.T) {
	dir := t.TempDir()

	output, err := runGenerateCmd(t, "--name", "pg-api", "--output", dir, "--db", "postgres", "--yes")
	if err != nil {
		t.Fatalf("generate --db postgres failed: %v\n%s", err, output)
	}

	project := filepath.Join(dir, "pg-api")
	for _, rel := range []string{"db/db.go", "docker-compose.yml"} {
		if _, err := os.Stat(filepath.Join(project, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
	if manifest, _ := os.ReadFile(filepath.Join(project, ".goforge.json")); !strings.Contains(string(manifest), `"database": "postgres"`) {
		t.Errorf("manifest should record the database:\n%s", manifest)
	}
	if !strings.Contains(output, "make db-up") {
		t.Errorf("next steps should start the database:\n%s", output)
	}
}

func TestGenerate_RejectsUnknownDatabase(t *testing.T) {
	_, err := runGenerateCmd(t, "--name", "x", "--output", t.TempDir(), "--db", "oracle", "--yes")
	if err == nil || !strings.Contains(err.Error(), "unsupported --db") {
		t.Errorf("error = %v, want unsupported --db", err)
	}
}

func TestGenerate_WithResources(t *testing.T) {
	dir := t.TempDir()
	var r result
	code := runJSONInto(t, &r, "generate", "--name", "shop", "--output", dir, "--db", "sqlite",
		"--resource", "category/categories:name:string", "--resource", "product:name:string(1..100),price:float(0..)")
	if code != 0 || !r.OK {
		t.Fatalf("exit %d, result %+v", code, r)
	}
	for _, want := range []string{"categories/category.go", "products/rules.go", "db/db.go"} {
		if !slices.Contains(r.Created, want) {
			t.Errorf("created does not list %s", want)
		}
	}
	if len(r.Updated) != 0 || len(r.Endpoints) != 12 || !strings.Contains(r.Summary, "with 2 resources") {
		t.Errorf("updated %v, %d endpoints, summary %q", r.Updated, len(r.Endpoints), r.Summary)
	}
	routes, _ := os.ReadFile(filepath.Join(dir, "shop", "server", "routes.go"))
	if !strings.Contains(string(routes), "categories.RegisterRoutes") || !strings.Contains(string(routes), "products.RegisterRoutes") {
		t.Errorf("routes.go doesn't register both resources:\n%s", routes)
	}
}

func TestGenerate_ResourceFailureCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"--resource", "product:price:float(5..1)"},                               // rejected before generating
		{"--resource", "product:name:string", "--resource", "product:sku:string"}, // fails after
	} {
		r, code := runJSON(t, append([]string{"generate", "--name", "shop", "--output", dir}, args...)...)
		if code != 1 || r.Error == nil {
			t.Fatalf("%v: exit %d, result %+v; want a failure", args, code, r)
		}
		if _, err := os.Stat(filepath.Join(dir, "shop")); !os.IsNotExist(err) {
			t.Errorf("%v: the project directory was left behind", args)
		}
	}
}
