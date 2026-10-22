package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo creates a repository in a temporary directory with CHANGELOG.md
// committed, makes it the working directory, and returns the commit.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, changelogFile), []byte(testChangelog), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, "init", "--quiet")
	git(t, "add", changelogFile)
	git(t, "commit", "--quiet", "-m", "release notes")
	return strings.TrimSpace(git(t, "rev-parse", "HEAD"))
}

func git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "tag.gpgSign=false", "-c", "commit.gpgSign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestGitTags(t *testing.T) {
	commit := gitRepo(t)
	git(t, "tag", "v1.1.0")                               // lightweight
	git(t, "tag", "-a", "v1.2.0", "-m", "GoForge v1.2.0") // annotated: its own object, pointing at the commit

	tags, err := gitTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags["v1.1.0"] != commit || tags["v1.2.0"] != commit {
		t.Errorf("gitTags() = %v, want both tags at the commit %s", tags, commit)
	}
}

func TestRun_Check(t *testing.T) {
	commit := gitRepo(t)
	git(t, "tag", "-a", "v1.2.0", "-m", "GoForge v1.2.0")

	var out bytes.Buffer
	if err := run([]string{"check", "release/1.3.0", commit}, &out); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if want := "version=1.3.0\ntag=v1.3.0\nprerelease=false\nresume=false\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}

	// The same version again, from another commit, is refused
	err := run([]string{"check", "release/1.2.0", "0000000"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already released") {
		t.Errorf("run() error = %v, want an already-released error", err)
	}
}

func TestRun_Notes(t *testing.T) {
	gitRepo(t)

	var out bytes.Buffer
	if err := run([]string{"notes", "v1.3.0"}, &out); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if out.String() != "### Added\n- Something new\n" {
		t.Errorf("output = %q", out.String())
	}
}

func TestRun_Usage(t *testing.T) {
	gitRepo(t)

	for _, args := range [][]string{nil, {"check", "release/1.3.0"}, {"publish", "v1.3.0"}} {
		if err := run(args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("run(%q) error = %v, want a usage error", args, err)
		}
	}
}

func TestRun_NoChangelog(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := run([]string{"notes", "v1.0.0"}, &bytes.Buffer{}); err == nil {
		t.Error("run() without CHANGELOG.md should fail")
	}
}
