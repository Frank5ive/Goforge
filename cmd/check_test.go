package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go vet and go test on a generated project")
	}
	project := generatedProject(t)

	var r checkResult
	if code := runJSONInto(t, &r, "check", "--dir", project); code != 0 || !r.OK || len(r.Problems) != 0 {
		t.Fatalf("a freshly generated project should pass: exit %d, %+v", code, r)
	}

	failing := "package handlers\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {\n\tt.Error(\"broken on purpose\")\n}\n"
	if err := os.WriteFile(filepath.Join(project, "handlers", "broken_test.go"), []byte(failing), 0o644); err != nil {
		t.Fatal(err)
	}

	r = checkResult{}
	if code := runJSONInto(t, &r, "check", "--dir", project); code != 1 || r.OK {
		t.Fatalf("exit %d, ok %t; want a failure", code, r.OK)
	}
	if len(r.Problems) != 1 || r.Problems[0].Test != "TestBroken" || r.Problems[0].File != "handlers/broken_test.go" || r.Problems[0].Line != 6 {
		t.Errorf("problems = %+v", r.Problems)
	}
}
