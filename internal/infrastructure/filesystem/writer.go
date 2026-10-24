// Package filesystem writes generated files to disk, and records every path
// written so commands can report them, or only records them in a dry run.
package filesystem

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Frank5ive/goforge/internal/domain"
)

// FileSystemWriter implements domain.FileWriter using Go's os package.
type FileSystemWriter struct{}

// NewFileSystemWriter creates a new instance of FileSystemWriter.
func NewFileSystemWriter() *FileSystemWriter {
	return &FileSystemWriter{}
}

// WriteFile implements domain.FileWriter.WriteFile.
// It writes content to a file at the specified path, creating parent directories if needed.
func (w *FileSystemWriter) WriteFile(path string, content string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := w.CreateDirectory(dir); err != nil {
		return domain.NewFileSystemError(
			"write",
			path,
			"failed to create parent directory",
			err,
		)
	}

	// Write file with appropriate permissions (0644 = rw-r--r--)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return domain.NewFileSystemError(
			"write",
			path,
			"failed to write file",
			err,
		)
	}

	return nil
}

// CreateDirectory implements domain.FileWriter.CreateDirectory.
// It creates a directory at the specified path, including any necessary parent directories.
func (w *FileSystemWriter) CreateDirectory(path string) error {
	// MkdirAll creates the directory and all parent directories with permissions 0755 (rwxr-xr-x)
	if err := os.MkdirAll(path, 0755); err != nil {
		return domain.NewFileSystemError(
			"mkdir",
			path,
			"failed to create directory",
			err,
		)
	}

	return nil
}

// FileExists implements domain.FileWriter.FileExists.
// It checks if a file or directory exists at the specified path.
func (w *FileSystemWriter) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// RemoveAll removes a file or directory and all its contents.
// This is useful for cleanup operations but not part of the domain interface.
func (w *FileSystemWriter) RemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return domain.NewFileSystemError(
			"remove",
			path,
			"failed to remove path",
			err,
		)
	}
	return nil
}

// ReadFile reads the content of a file at the specified path.
// This is useful for testing but not part of the domain interface.
func (w *FileSystemWriter) ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", domain.NewFileSystemError(
			"read",
			path,
			"failed to read file",
			err,
		)
	}
	return string(content), nil
}

// ListDirectory lists all files and directories in the specified path.
// This is useful for debugging but not part of the domain interface.
func (w *FileSystemWriter) ListDirectory(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, domain.NewFileSystemError(
			"list",
			path,
			"failed to list directory",
			err,
		)
	}

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}

	return names, nil
}

// GetAbsolutePath returns the absolute path for the given path.
func (w *FileSystemWriter) GetAbsolutePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	return absPath, nil
}
