package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspect(t *testing.T) {
	project := generatedProject(t)
	if _, code := runJSON(t, "add", "resource", "product", "--dir", project, "--fields", "name:string,notes:string?"); code != 0 {
		t.Fatal("add resource failed")
	}
	if _, code := runJSON(t, "add", "db", "sqlite", "--dir", project); code != 0 {
		t.Fatal("add db failed")
	}

	var r inspectResult
	if code := runJSONInto(t, &r, "inspect", "--dir", project); code != 0 || !r.OK || r.Command != "inspect" {
		t.Fatalf("exit %d, result %+v", code, r)
	}

	if r.AppName != "shop" || r.Database != "sqlite" || r.GoVersion == "" || len(r.Resources) != 1 {
		t.Errorf("project = %+v", r.ProjectInfo)
	}
	if res := r.Resources[0]; res.Path != "/products" || res.Package != "products" || len(res.Fields) != 2 || !res.Fields[1].Optional {
		t.Errorf("resource = %+v", res)
	}
	if len(r.Routes) != 8 { // /health, /readyz, and six for the resource
		t.Errorf("routes = %+v", r.Routes)
	}
	if len(r.Problems) != 0 {
		t.Errorf("problems = %v, want none", r.Problems)
	}

	// Every variable inspect reports must be read by the generated code
	var code strings.Builder
	for _, file := range []string{"config/config.go", "logger/logger.go"} {
		src, err := os.ReadFile(filepath.Join(project, file))
		if err != nil {
			t.Fatal(err)
		}
		code.Write(src)
	}
	for _, name := range r.Env {
		if !strings.Contains(code.String(), `"`+name+`"`) {
			t.Errorf("inspect reports %s, but the generated code doesn't read it", name)
		}
	}
}

func TestInspect_ReportsMissingMarker(t *testing.T) {
	project := generatedProject(t)
	routes := filepath.Join(project, "server", "routes.go")
	src, err := os.ReadFile(routes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routes, []byte(strings.ReplaceAll(string(src), "// goforge:routes", "")), 0o644); err != nil {
		t.Fatal(err)
	}

	var r inspectResult
	runJSONInto(t, &r, "inspect", "--dir", project)
	if len(r.Problems) != 1 || !strings.Contains(r.Problems[0], "goforge:routes") {
		t.Errorf("problems = %v, want the missing routes marker", r.Problems)
	}
}

func TestInspect_NotAProject(t *testing.T) {
	r, code := runJSON(t, "inspect", "--dir", t.TempDir())
	if code != 1 || r.Error == nil || r.Error.Code != "not_a_goforge_project" {
		t.Errorf("exit %d, error %+v", code, r.Error)
	}
}
