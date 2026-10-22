package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestGenerate_PromptsRequireTerminal(t *testing.T) {
	// go test's stdin is not a terminal, so prompting must fail instead of hanging
	_, err := runGenerateCmd(t, "--name", "prompt-api", "--output", t.TempDir())

	if !errors.Is(err, errNotInteractive) {
		t.Errorf("Expected errNotInteractive, got %v", err)
	}
}

func TestGenerate_NoPromptWhenAllOptionsSetByFlag(t *testing.T) {
	dir := t.TempDir()

	output, err := runGenerateCmd(t, "--name", "all-flags", "--output", dir,
		"--module", "example.com/all-flags", "--logger=false", "--docker=false", "--readme=false", "--ci=false")
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
