package gocheck

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

// The outputs below were captured from the real tools on a generated project.

func TestParseVet_CompileError(t *testing.T) {
	stderr := `# github.com/you/shop/products
products/handler.go:72:21: h.svc.Get undefined (type *Service has no field or method Get)
# github.com/you/shop/products
# [github.com/you/shop/products]
vet: products/handler.go:72:21: h.svc.Get undefined (type *Service has no field or method Get)
`
	problems, compileFailed := parseVet([]byte(stderr))

	want := domain.CheckProblem{Step: "vet", File: "products/handler.go", Line: 72, Column: 21,
		Message: "h.svc.Get undefined (type *Service has no field or method Get)"}
	if len(problems) != 1 || problems[0] != want || !compileFailed {
		t.Errorf("parseVet() = %+v, %t; want the error once, as a compile failure", problems, compileFailed)
	}
}

func TestParseVet_VetFinding(t *testing.T) {
	stderr := `# github.com/you/shop/products
# [github.com/you/shop/products]
products/bad.go:5:14: fmt.Printf format %d has arg "x" of wrong type string
`
	problems, compileFailed := parseVet([]byte(stderr))
	if len(problems) != 1 || problems[0].File != "products/bad.go" || problems[0].Line != 5 || compileFailed {
		t.Errorf("parseVet() = %+v, %t", problems, compileFailed)
	}
}

func TestParseTestJSON(t *testing.T) {
	absDir := filepath.Join(t.TempDir(), "my shop") // a space, as in "C:\Users\Jane Doe"
	stack, _ := json.Marshal("\t" + filepath.Join(absDir, "products", "zz_test.go") + ":11 +0x28\n")
	stdout := strings.Join([]string{
		// A failing assertion
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestFails","Output":"=== RUN   TestFails\n"}`,
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestFails","Output":"    zz_test.go:6: got \"a\", want \"b\"\n"}`,
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestFails","Output":"--- FAIL: TestFails (0.00s)\n"}`,
		`{"Action":"fail","Package":"github.com/you/shop/products","Test":"TestFails"}`,
		// A panic, located by its stack
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestPanics","Output":"--- FAIL: TestPanics (0.00s)\n"}`,
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestPanics","Output":"panic: assignment to entry in nil map [recovered]\n"}`,
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestPanics","Output":"\t/usr/lib/go/src/testing/testing.go:1734 +0x21c\n"}`,
		`{"Action":"output","Package":"github.com/you/shop/products","Test":"TestPanics","Output":` + string(stack) + `}`,
		`{"Action":"fail","Package":"github.com/you/shop/products","Test":"TestPanics"}`,
		// A failing subtest: only the subtest is reported, not its parent
		`{"Action":"output","Package":"github.com/you/shop","Test":"TestRoutes/health","Output":"    server_test.go:40: GET /health = 500, want 200\n"}`,
		`{"Action":"fail","Package":"github.com/you/shop","Test":"TestRoutes/health"}`,
		`{"Action":"output","Package":"github.com/you/shop","Test":"TestRoutes","Output":"--- FAIL: TestRoutes (0.00s)\n"}`,
		`{"Action":"fail","Package":"github.com/you/shop","Test":"TestRoutes"}`,
		// Package failures explained by their tests are not reported again
		`{"Action":"output","Package":"github.com/you/shop/products","Output":"FAIL\tgithub.com/you/shop/products\t0.006s\n"}`,
		`{"Action":"fail","Package":"github.com/you/shop/products"}`,
		`not JSON: go: downloading something`,
	}, "\n")

	problems := parseTestJSON([]byte(stdout), absDir, "github.com/you/shop")

	want := []domain.CheckProblem{
		{Step: "test", Package: "github.com/you/shop/products", Test: "TestFails", File: "products/zz_test.go", Line: 6,
			Message: `zz_test.go:6: got "a", want "b"`},
		{Step: "test", Package: "github.com/you/shop/products", Test: "TestPanics", File: "products/zz_test.go", Line: 11,
			Message: "panic: assignment to entry in nil map"},
		{Step: "test", Package: "github.com/you/shop", Test: "TestRoutes/health", File: "server_test.go", Line: 40,
			Message: "server_test.go:40: GET /health = 500, want 200"},
	}
	if len(problems) != len(want) {
		t.Fatalf("parseTestJSON() = %+v, want %+v", problems, want)
	}
	for i := range want {
		if problems[i] != want[i] {
			t.Errorf("problem %d = %+v, want %+v", i, problems[i], want[i])
		}
	}
}

func TestParseTestJSON_BuildFailure(t *testing.T) {
	stdout := `{"ImportPath":"github.com/you/shop/products [github.com/you/shop/products.test]","Action":"build-output","Output":"# github.com/you/shop/products [github.com/you/shop/products.test]\n"}
{"ImportPath":"github.com/you/shop/products [github.com/you/shop/products.test]","Action":"build-output","Output":"products/zz_test.go:6:4: t.Nope undefined (type *testing.T has no field or method Nope)\n"}
{"ImportPath":"github.com/you/shop/products [github.com/you/shop/products.test]","Action":"build-fail"}
{"Action":"output","Package":"github.com/you/shop/products","Output":"FAIL\tgithub.com/you/shop/products [build failed]\n"}
{"Action":"fail","Package":"github.com/you/shop/products"}`

	problems := parseTestJSON([]byte(stdout), "/work/shop", "github.com/you/shop")
	if len(problems) != 1 || problems[0].File != "products/zz_test.go" || problems[0].Line != 6 || problems[0].Column != 4 {
		t.Errorf("parseTestJSON() = %+v, want just the compile error", problems)
	}
}

func TestParseLintJSON(t *testing.T) {
	stdout := `{"Issues":[{"FromLinter":"errcheck","Text":"Error return value of ` + "`os.Remove`" + ` is not checked","Pos":{"Filename":"products/bad.go","Offset":53,"Line":5,"Column":23}}],"Report":{}}`

	problems, err := parseLintJSON([]byte(stdout))
	want := domain.CheckProblem{Step: "lint", File: "products/bad.go", Line: 5, Column: 23, Linter: "errcheck",
		Message: "Error return value of `os.Remove` is not checked"}
	if err != nil || len(problems) != 1 || problems[0] != want {
		t.Errorf("parseLintJSON() = %+v, %v", problems, err)
	}
}

func TestParseGofmt(t *testing.T) {
	problems := parseGofmt([]byte("main.go\nproducts/service.go\n"))
	if len(problems) != 2 || problems[1].File != "products/service.go" || !strings.Contains(problems[1].Message, "gofmt -w products/service.go") {
		t.Errorf("parseGofmt() = %+v", problems)
	}
}

// fakeTools answers each command with canned output.
type fakeTools struct {
	vetStderr string
	installed map[string]bool
	ran       []string
}

func (f *fakeTools) run(_ context.Context, _, name string, args ...string) ([]byte, []byte, error) {
	f.ran = append(f.ran, filepath.Base(name)+" "+args[0])
	if len(args) > 0 && args[0] == "vet" && f.vetStderr != "" {
		return nil, []byte(f.vetStderr), errors.New("exit status 1")
	}
	if filepath.Base(name) == "golangci-lint" {
		return []byte(`{"Issues":[]}`), nil, nil
	}
	return nil, nil, nil
}

func (f *fakeTools) lookPath(name string) (string, error) {
	if f.installed[name] {
		return "/usr/bin/" + name, nil
	}
	return "", exec.ErrNotFound
}

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/shop\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheck_SkipsTestsAndLintWhenTheCodeDoesNotCompile(t *testing.T) {
	tools := &fakeTools{
		vetStderr: "vet: main.go:3:1: undefined: x\n",
		installed: map[string]bool{"go": true, "gofmt": true, "golangci-lint": true},
	}
	report, err := (&Checker{run: tools.run, lookPath: tools.lookPath}).Check(t.Context(), project(t), domain.CheckOptions{})
	if err != nil {
		t.Fatal(err)
	}

	statuses := map[string]string{}
	for _, s := range report.Steps {
		statuses[s.Name] = s.Status
	}
	want := map[string]string{"format": "passed", "vet": "failed", "test": "skipped", "lint": "skipped"}
	for name, status := range want {
		if statuses[name] != status {
			t.Errorf("%s = %q, want %q (steps: %+v)", name, statuses[name], status, report.Steps)
		}
	}
	if report.Passed() || len(report.Problems) != 1 {
		t.Errorf("report = %+v", report)
	}
}

func TestCheck_SkipsLintWithoutGolangciLint(t *testing.T) {
	tools := &fakeTools{installed: map[string]bool{"go": true, "gofmt": true}}
	report, err := (&Checker{run: tools.run, lookPath: tools.lookPath}).Check(t.Context(), project(t), domain.CheckOptions{Race: true})
	if err != nil {
		t.Fatal(err)
	}

	last := report.Steps[len(report.Steps)-1]
	if !report.Passed() || last.Name != "lint" || last.Status != "skipped" || !strings.Contains(last.Note, "not installed") {
		t.Errorf("steps = %+v", report.Steps)
	}
	if strings.Join(tools.ran, ", ") != "gofmt -l, go vet, go test" {
		t.Errorf("ran %v", tools.ran)
	}
}

func TestCheck_NotAModule(t *testing.T) {
	_, err := New().Check(t.Context(), t.TempDir(), domain.CheckOptions{})
	if domain.ErrorCode(err) != domain.CodeNotAProject {
		t.Errorf("Check() error = %v, want a not_a_goforge_project error", err)
	}
}

func TestStackLocation(t *testing.T) {
	// t.TempDir is itself behind a symlink on macOS, and may use short names on
	// Windows; stack traces may spell the project directory either way
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, file := range []string{"main.go", filepath.Join("products", "zz_test.go")} {
		path := filepath.Join(real, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		frame, absDir, file string
		line                int
	}{
		{filepath.Join(real, "products", "zz_test.go") + ":11 +0x28", real, "products/zz_test.go", 11},
		// The project is reached through a symlink; the stack shows another spelling
		{filepath.Join(real, "main.go") + ":7 +0x1f", link, "main.go", 7},
		{filepath.Join(resolved, "main.go") + ":7 +0x1f", link, "main.go", 7},
		{filepath.Join(resolved, "main.go") + ":7 +0x1f", real, "main.go", 7},
		{"/usr/lib/go/src/testing/testing.go:1734 +0x21c", real, "", 0},
		{"created by testing.(*T).Run in goroutine 1", real, "", 0},
	}
	for _, tt := range tests {
		if file, line := stackLocation(tt.frame, tt.absDir); file != tt.file || line != tt.line {
			t.Errorf("stackLocation(%q) = %q, %d; want %q, %d", tt.frame, file, line, tt.file, tt.line)
		}
	}
}
