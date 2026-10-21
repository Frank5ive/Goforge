package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordingWriter_WritesAndRecords(t *testing.T) {
	dir := t.TempDir()
	writer := NewRecordingWriter(NewFileSystemWriter(), false)

	path := filepath.Join(dir, "sub", "file.txt")
	if err := writer.WriteFile(path, "hello"); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if content, err := os.ReadFile(path); err != nil || string(content) != "hello" {
		t.Errorf("Expected file to be written with content, got %q, err %v", content, err)
	}

	if files := writer.Files(); len(files) != 1 || files[0] != path {
		t.Errorf("Expected recorded files [%s], got %v", path, files)
	}
}

func TestRecordingWriter_DryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	writer := NewRecordingWriter(NewFileSystemWriter(), true)

	project := filepath.Join(dir, "project")
	if err := writer.CreateDirectory(project); err != nil {
		t.Fatalf("CreateDirectory failed: %v", err)
	}
	if err := writer.WriteFile(filepath.Join(project, "main.go"), "package main"); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Errorf("Dry run must not create %s", project)
	}

	if files := writer.Files(); len(files) != 1 {
		t.Errorf("Expected dry run to record 1 file, got %v", files)
	}
}

func TestRecordingWriter_DryRunChecksRealFilesystem(t *testing.T) {
	dir := t.TempDir()
	writer := NewRecordingWriter(NewFileSystemWriter(), true)

	if !writer.FileExists(dir) {
		t.Error("Expected FileExists to see the real directory during a dry run")
	}

	if err := writer.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("Dry run RemoveAll must not delete anything")
	}
}
