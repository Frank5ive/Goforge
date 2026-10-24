package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

// runText runs goforge with args through execute, as the binary does, and
// returns its output.
func runText(t *testing.T, args ...string) (string, int) {
	t.Helper()

	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	code := execute(root, args)
	return out.String(), code
}

func assertContains(t *testing.T, output string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(output, w) {
			t.Errorf("output is missing %q:\n%s", w, output)
		}
	}
}

func TestRootShowsQuickStart(t *testing.T) {
	output, code := runText(t)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	assertContains(t, output, "Quick Start:", "goforge generate --name=my-api", "github.com/Frank5ive/goforge")
}

func TestInspectText(t *testing.T) {
	project := generatedProject(t)

	output, code := runText(t, "inspect", "--dir", project)
	if code != 0 {
		t.Fatalf("exit code = %d:\n%s", code, output)
	}
	assertContains(t, output, "🔎 shop", "Database: none (in memory)", "📦 Resources (0):", "none yet",
		"GET    /health", "Environment: APP_ENV, PORT", "Markers: 7 of 7 present", "✅ No problems")

	if _, code := runJSON(t, "add", "resource", "product", "--dir", project, "--fields", "name:string,notes:string?"); code != 0 {
		t.Fatal("add resource failed")
	}
	if err := os.Remove(filepath.Join(project, "server", "routes.go")); err != nil {
		t.Fatal(err)
	}

	output, _ = runText(t, "inspect", "--dir", project)
	assertContains(t, output, "📦 Resources (1):", "/products", "name:string, notes:string?",
		"⚠️  server/routes.go is missing")
}

func TestCheckReportText(t *testing.T) {
	report := domain.CheckReport{
		Steps: []domain.CheckStep{
			{Name: "format", Status: domain.StepPassed, DurationMS: 40},
			{Name: "vet", Status: domain.StepPassed, DurationMS: 1200},
			{Name: "test", Status: domain.StepFailed, Problems: 1, DurationMS: 600},
			{Name: "lint", Status: domain.StepFailed, Problems: 1, DurationMS: 2000},
		},
		Problems: []domain.CheckProblem{
			{Step: "test", File: "products/service_test.go", Line: 42, Test: "TestCreate", Message: "name = \"\"\nwant \"widget\""},
			{Step: "lint", File: "main.go", Line: 7, Linter: "errcheck", Message: "unchecked error"},
		},
	}

	var out bytes.Buffer
	printCheckReport(&out, report)
	assertContains(t, out.String(),
		"✓ format  0s", "✓ vet     1.2s", "✗ test    600ms  1 problem", "✗ lint    2s  1 problem",
		"❌ products/service_test.go:42  TestCreate", "   name = \"\"\n   want \"widget\"",
		"❌ main.go:7  lint (errcheck)", "2 problems. Fix them and run `goforge check` again.")

	out.Reset()
	printCheckReport(&out, domain.CheckReport{
		Steps:    []domain.CheckStep{{Name: "vet", Status: domain.StepFailed, Problems: 1}, {Name: "test", Status: domain.StepSkipped, Note: "the code does not compile"}},
		Problems: []domain.CheckProblem{{Step: "vet", File: "main.go", Line: 3, Message: "undefined: x"}},
	})
	assertContains(t, out.String(), "- test    skipped: the code does not compile", "❌ main.go:3  vet", "1 problem. Fix it")

	out.Reset()
	printCheckReport(&out, domain.CheckReport{Steps: []domain.CheckStep{{Name: "format", Status: domain.StepPassed}}})
	assertContains(t, out.String(), "✅ All checks passed.")
}

func TestCheckText(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go vet and go test on a generated project")
	}
	project := generatedProject(t)

	output, code := runText(t, "check", "--dir", project)
	if code != 0 {
		t.Fatalf("exit code = %d:\n%s", code, output)
	}
	assertContains(t, output, "🔍 Checking", "✓ format", "✓ vet", "✓ test", "✅ All checks passed.")
}
