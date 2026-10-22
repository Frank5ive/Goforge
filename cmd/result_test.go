package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// runJSON runs goforge with args (plus --json) through Execute, as the binary
// does, and decodes the JSON result it prints.
func runJSON(t *testing.T, args ...string) (result, int) {
	t.Helper()

	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	code := execute(root, append(args, "--json"))

	var r result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("output is not a JSON result: %v\n%s", err, out.String())
	}
	return r, code
}

func TestJSON_Generate(t *testing.T) {
	dir := t.TempDir()
	r, code := runJSON(t, "generate", "--name", "shop", "--output", dir)

	if code != 0 || !r.OK || r.Command != "generate" || r.Error != nil {
		t.Fatalf("exit %d, result %+v", code, r)
	}
	if r.Project != filepath.Join(dir, "shop") {
		t.Errorf("project = %q, want the absolute project path", r.Project)
	}
	for _, want := range []string{"main.go", "server/server.go", ".goforge.json"} {
		if !slices.Contains(r.Created, want) {
			t.Errorf("created does not list %s: %v", want, r.Created)
		}
	}
	if len(r.NextSteps) == 0 {
		t.Error("expected next steps")
	}
}

func TestJSON_AddResource(t *testing.T) {
	project := generatedProject(t)
	r, code := runJSON(t, "add", "resource", "product", "--dir", project, "--fields", "name:string,price:float")

	if code != 0 || !r.OK || r.Command != "add resource" {
		t.Fatalf("exit %d, result %+v", code, r)
	}
	if !slices.Contains(r.Created, "products/product.go") || !slices.Contains(r.Updated, "server/routes.go") {
		t.Errorf("created %v, updated %v", r.Created, r.Updated)
	}
	if len(r.Endpoints) != 6 || r.Endpoints[0] != (endpoint{Method: "GET", Path: "/products", Query: "limit=20&offset=0"}) {
		t.Errorf("endpoints = %+v", r.Endpoints)
	}
}

func TestJSON_DryRun(t *testing.T) {
	project := generatedProject(t)
	r, code := runJSON(t, "add", "db", "sqlite", "--dir", project, "--dry-run")

	if code != 0 || !r.OK || !r.DryRun || len(r.Created) == 0 {
		t.Fatalf("exit %d, result %+v", code, r)
	}
	if _, err := os.Stat(filepath.Join(project, "db")); !os.IsNotExist(err) {
		t.Error("a dry run wrote files")
	}
}

func TestJSON_ErrorCodes(t *testing.T) {
	project := generatedProject(t)
	if _, code := runJSON(t, "add", "resource", "product", "--dir", project); code != 0 {
		t.Fatal("adding the first resource failed")
	}

	tests := []struct {
		name string
		args []string
		code string
	}{
		{"missing argument", []string{"add", "resource"}, codeUsage},
		{"unknown flag", []string{"add", "resource", "x", "--nope"}, codeUsage},
		{"invalid field", []string{"add", "resource", "order", "--dir", project, "--fields", "total:money"}, "invalid_input"},
		{"unsupported database", []string{"generate", "--name", "x", "--output", t.TempDir(), "--db", "oracle"}, "invalid_input"},
		{"not a project", []string{"add", "resource", "order", "--dir", t.TempDir()}, "not_a_goforge_project"},
		{"resource exists", []string{"add", "resource", "product", "--dir", project}, "already_exists"},
		{"project exists", []string{"generate", "--name", filepath.Base(project), "--output", filepath.Dir(project)}, "already_exists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, code := runJSON(t, tt.args...)
			if code != 1 || r.OK || r.Error == nil {
				t.Fatalf("exit %d, result %+v; want a failure", code, r)
			}
			if r.Error.Code != tt.code || r.Error.Message == "" || r.Error.Hint == "" {
				t.Errorf("error = %+v, want code %q with a message and hint", r.Error, tt.code)
			}
		})
	}
}

func TestErrorText(t *testing.T) {
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)

	if code := execute(root, []string{"add", "resource", "order", "--dir", t.TempDir()}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	for _, want := range []string{"❌ Error:", "💡 Run the command inside a project"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
}

func TestResultText(t *testing.T) {
	r := newResult("add resource", ".", false)
	r.Summary = "Added the product resource"
	r.Created = []string{"products/product.go"}
	r.NextSteps = []nextStep{{Command: "go test ./products", Description: "run its tests"}, {Command: "make run", Description: "start"}}

	var out bytes.Buffer
	if err := r.print(&out, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"✅ Added the product resource", "Created 1 files:", "   products/product.go", "   go test ./products  # run its tests", "   make run            # start"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
}
