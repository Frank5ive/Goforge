package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

func TestGenerate_WithResourcesAndGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()

	out, err := runGenerateCmd(t, "-n", "shop", "-o", dir, "--yes", "--git",
		"--resource", "category/categories:name:string",
		"--resource", "product:name:string(1..100),category_id:ref(category)")
	if err != nil {
		t.Fatalf("generate error = %v\n%s", err, out)
	}

	project := filepath.Join(dir, "shop")
	for _, path := range []string{"categories/service.go", "products/service.go", ".git"} {
		if _, err := os.Stat(filepath.Join(project, path)); err != nil {
			t.Errorf("%s was not created: %v", path, err)
		}
	}
	for _, want := range []string{"with 2 resources", "/categories", "/products", "Initialized a git repository"} {
		if !strings.Contains(out, want) {
			t.Errorf("output doesn't mention %q:\n%s", want, out)
		}
	}
}

func TestGenerate_WithOneResourceSaysSoInTheSingular(t *testing.T) {
	out, err := runGenerateCmd(t, "-n", "shop", "-o", t.TempDir(), "--yes", "--resource", "product:name:string")
	if err != nil || !strings.Contains(out, "with 1 resource") || strings.Contains(out, "1 resources") {
		t.Errorf("output = %q, err = %v; want %q", out, err, "with 1 resource")
	}
}

func TestGenerate_RejectsBadResourceSpecs(t *testing.T) {
	for _, spec := range []string{"nofields", "Product:name:string", "product:name:blob", "product:name:string(5..1)"} {
		dir := t.TempDir()
		_, err := runGenerateCmd(t, "-n", "shop", "-o", dir, "--yes", "--resource", spec)
		if err == nil {
			t.Errorf("--resource %q was accepted", spec)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "shop")); !os.IsNotExist(statErr) {
			t.Errorf("--resource %q created the project before failing", spec)
		}
	}
}

func TestGenerate_WithoutGitSaysSoInANote(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no git here
	out, err := runGenerateCmd(t, "-n", "shop", "-o", t.TempDir(), "--yes", "--git")
	if err != nil || !strings.Contains(out, "The git repository was not initialized") {
		t.Errorf("output = %q, err = %v; want a note that git was not initialized", out, err)
	}
}

func TestInitGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	if err := initGitRepo(filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(err.Error(), "git init") {
		t.Errorf("initGitRepo() in a missing directory error = %v, want a git init error", err)
	}

	dir := t.TempDir()
	if err := initGitRepo(dir); err != nil {
		t.Fatalf("initGitRepo() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("no .git directory after initGitRepo(): %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if err := initGitRepo(dir); err == nil || !strings.Contains(err.Error(), "git not found") {
		t.Errorf("initGitRepo() without git error = %v, want %q", err, "git not found")
	}
}

func TestExecute(t *testing.T) {
	args := os.Args
	t.Cleanup(func() { os.Args = args })

	os.Args = []string{"goforge", "--version"}
	if code := Execute(); code != 0 {
		t.Errorf("Execute() --version = %d, want 0", code)
	}
	os.Args = []string{"goforge", "no-such-command"}
	if code := Execute(); code != 1 {
		t.Errorf("Execute() with an unknown command = %d, want 1", code)
	}
}

func TestCommandNameAndHint(t *testing.T) {
	if got := commandName(nil); got != "" {
		t.Errorf("commandName(nil) = %q, want empty", got)
	}
	root := newRootCmd()
	sub, _, err := root.Find([]string{"add", "resource"})
	if err != nil {
		t.Fatal(err)
	}
	if got := commandName(sub); got != "add resource" {
		t.Errorf("commandName(add resource) = %q", got)
	}

	// A hint on the error wins; otherwise the code's own hint is used
	own := domain.WithHint(domain.CodeInvalidInput, "try this instead", os.ErrInvalid)
	if got := hint(own, domain.CodeInvalidInput); got != "try this instead" {
		t.Errorf("hint() = %q, want the error's own hint", got)
	}
	if got := hint(os.ErrInvalid, domain.CodeWriteFailed); got != hints[domain.CodeWriteFailed] {
		t.Errorf("hint() = %q, want the hint for the code", got)
	}
	if got := hint(os.ErrInvalid, "unknown_code"); got != "" {
		t.Errorf("hint() for an unknown code = %q, want empty", got)
	}
}
