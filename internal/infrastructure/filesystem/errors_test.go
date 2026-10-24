package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

// notADirectory returns a path under a regular file, so nothing can be
// created there.
func notADirectory(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(file, "child")
}

func TestWriterReportsFailures(t *testing.T) {
	w := NewFileSystemWriter()
	blocked := notADirectory(t)

	if err := w.CreateDirectory(blocked); err == nil {
		t.Error("CreateDirectory() under a file = nil, want an error")
	}
	if err := w.WriteFile(filepath.Join(blocked, "a.txt"), "x"); err == nil {
		t.Error("WriteFile() under a file = nil, want an error")
	}
	if err := w.WriteFile(t.TempDir(), "x"); err == nil {
		t.Error("WriteFile() onto a directory = nil, want an error")
	}
}

func TestWriterRemoveAll(t *testing.T) {
	w := NewFileSystemWriter()
	dir := filepath.Join(t.TempDir(), "a", "b")
	if err := w.WriteFile(filepath.Join(dir, "f.txt"), "x"); err != nil {
		t.Fatal(err)
	}

	if err := w.RemoveAll(filepath.Dir(dir)); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
	if w.FileExists(dir) {
		t.Error("the directory still exists after RemoveAll()")
	}
	// Removing what isn't there is not an error
	if err := w.RemoveAll(dir); err != nil {
		t.Errorf("RemoveAll() of a missing path error = %v, want nil", err)
	}
}

func TestRecordingWriterDryRunAndRemoveAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	dry := NewRecordingWriter(NewFileSystemWriter(), true)
	if !dry.DryRun() {
		t.Error("DryRun() = false for a dry-run writer")
	}
	if err := dry.RemoveAll(dir); err != nil || !dry.FileExists(dir) {
		t.Errorf("a dry run removed %s (err %v)", dir, err)
	}

	real := NewRecordingWriter(NewFileSystemWriter(), false)
	if real.DryRun() {
		t.Error("DryRun() = true for a real writer")
	}
	if err := real.RemoveAll(dir); err != nil || real.FileExists(dir) {
		t.Errorf("RemoveAll() left %s (err %v)", dir, err)
	}
}
