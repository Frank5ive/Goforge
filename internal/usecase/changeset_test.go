package usecase

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Frank5ive/goforge/internal/infrastructure/filesystem"
)

func TestChangeset_ApplyWritesEverything(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := newChangeset(filesystem.NewFileSystemWriter())
	if err := cs.create(filepath.Join(dir, "a", "b", "new.txt"), "new"); err != nil {
		t.Fatal(err)
	}
	cs.edit(existing, "updated")

	if err := cs.apply(nil, nil); err != nil {
		t.Fatalf("apply() error = %v", err)
	}
	assertContent(t, filepath.Join(dir, "a", "b", "new.txt"), "new")
	assertContent(t, existing, "updated")
}

func TestChangeset_CreateRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "taken.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := newChangeset(filesystem.NewFileSystemWriter()).create(path, "y"); err == nil {
		t.Error("create() of an existing file should fail")
	}
}

func TestChangeset_RollbackRestoresEverything(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	goMod := filepath.Join(dir, "go.mod")
	for path, content := range map[string]string{existing: "old", goMod: "module old"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cs := newChangeset(filesystem.NewFileSystemWriter())
	if err := cs.create(filepath.Join(dir, "db", "migrations", "1.sql"), "sql"); err != nil {
		t.Fatal(err)
	}
	cs.edit(existing, "updated")

	err := cs.apply([]string{goMod}, func() error {
		// Simulate `go get` changing go.mod and then failing
		_ = os.WriteFile(goMod, []byte("module changed"), 0o644)
		return errors.New("go get failed")
	})
	if err == nil {
		t.Fatal("apply() error = nil, want the after() error")
	}

	assertContent(t, existing, "old")
	assertContent(t, goMod, "module old")
	if _, err := os.Stat(filepath.Join(dir, "db")); !os.IsNotExist(err) {
		t.Error("rollback must remove directories the changeset created")
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func TestChangeset_RollbackRemovesRestorePathsCreatedDuringApply(t *testing.T) {
	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	goSum := filepath.Join(dir, "go.sum") // a stdlib-only project has none yet
	if err := os.WriteFile(goMod, []byte("module old"), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := newChangeset(filesystem.NewFileSystemWriter())
	err := cs.apply([]string{goMod, goSum}, func() error {
		// Simulate `go get` succeeding (creating go.sum) and `go mod tidy` failing
		_ = os.WriteFile(goMod, []byte("module changed"), 0o644)
		_ = os.WriteFile(goSum, []byte("example.com/x v1.0.0 h1:abc"), 0o644)
		return errors.New("go mod tidy failed")
	})
	if err == nil {
		t.Fatal("apply() error = nil, want the after() error")
	}

	assertContent(t, goMod, "module old")
	if _, err := os.Stat(goSum); !os.IsNotExist(err) {
		t.Error("rollback must remove go.sum, which did not exist before apply")
	}
}
