package filesystem

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewFileSystemWriter(t *testing.T) {
	writer := NewFileSystemWriter()
	if writer == nil {
		t.Fatal("Expected writer to be created, got nil")
	}
}

func TestFileSystemWriter_WriteFile(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	testPath := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!"

	err := writer.WriteFile(testPath, testContent)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify file was created
	if !writer.FileExists(testPath) {
		t.Error("Expected file to exist")
	}

	// Verify content
	content, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}
}

func TestFileSystemWriter_WriteFile_CreatesParentDirs(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	// Create a nested path
	testPath := filepath.Join(tempDir, "a", "b", "c", "test.txt")
	testContent := "nested file"

	err := writer.WriteFile(testPath, testContent)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify file was created
	if !writer.FileExists(testPath) {
		t.Error("Expected file to exist")
	}

	// Verify parent directories were created
	parentDir := filepath.Join(tempDir, "a", "b", "c")
	if !writer.FileExists(parentDir) {
		t.Error("Expected parent directory to exist")
	}
}

func TestFileSystemWriter_CreateDirectory(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	testDir := filepath.Join(tempDir, "testdir")

	err := writer.CreateDirectory(testDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify directory was created
	if !writer.FileExists(testDir) {
		t.Error("Expected directory to exist")
	}

	// Verify it's actually a directory
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("Expected path to be a directory")
	}
}

func TestFileSystemWriter_CreateDirectory_Nested(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	testDir := filepath.Join(tempDir, "a", "b", "c", "d")

	err := writer.CreateDirectory(testDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify all nested directories were created
	if !writer.FileExists(testDir) {
		t.Error("Expected nested directory to exist")
	}
}

func TestFileSystemWriter_FileExists_True(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "exists.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if !writer.FileExists(testFile) {
		t.Error("Expected FileExists to return true")
	}
}

func TestFileSystemWriter_FileExists_False(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "nonexistent.txt")

	if writer.FileExists(testFile) {
		t.Error("Expected FileExists to return false")
	}
}

func TestFileSystemWriter_FileExists_Directory(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	// Create a test directory
	testDir := filepath.Join(tempDir, "testdir")
	err := os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	if !writer.FileExists(testDir) {
		t.Error("Expected FileExists to return true for directory")
	}
}

func TestFileSystemWriter_RemoveAll(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	// Create a test directory with files
	testDir := filepath.Join(tempDir, "remove_test")
	err := writer.CreateDirectory(testDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	testFile := filepath.Join(testDir, "file.txt")
	err = writer.WriteFile(testFile, "content")
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Remove the directory
	err = writer.RemoveAll(testDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify it was removed
	if writer.FileExists(testDir) {
		t.Error("Expected directory to be removed")
	}
}

func TestFileSystemWriter_ReadFile(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	testPath := filepath.Join(tempDir, "read_test.txt")
	testContent := "Read this content"

	// Write file first
	err := writer.WriteFile(testPath, testContent)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Read it back
	content, err := writer.ReadFile(testPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if content != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, content)
	}
}

func TestFileSystemWriter_ReadFile_NonExistent(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	testPath := filepath.Join(tempDir, "nonexistent.txt")

	_, err := writer.ReadFile(testPath)
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}
}

func TestFileSystemWriter_ListDirectory(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	// Create some test files
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, fileName := range testFiles {
		path := filepath.Join(tempDir, fileName)
		err := writer.WriteFile(path, "content")
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// List directory
	entries, err := writer.ListDirectory(tempDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(entries) != len(testFiles) {
		t.Errorf("Expected %d entries, got %d", len(testFiles), len(entries))
	}
}

func TestFileSystemWriter_ListDirectory_Empty(t *testing.T) {
	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	entries, err := writer.ListDirectory(tempDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

func TestFileSystemWriter_GetAbsolutePath(t *testing.T) {
	writer := NewFileSystemWriter()

	// Test with current directory
	absPath, err := writer.GetAbsolutePath(".")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if absPath == "" {
		t.Error("Expected non-empty absolute path")
	}

	// Verify it's actually absolute
	if !filepath.IsAbs(absPath) {
		t.Errorf("Expected absolute path, got: %s", absPath)
	}
}

func TestFileSystemWriter_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}

	writer := NewFileSystemWriter()
	tempDir := t.TempDir()

	// Test file permissions (0644)
	testFile := filepath.Join(tempDir, "test.txt")
	err := writer.WriteFile(testFile, "content")
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode()
	expectedMode := os.FileMode(0644)

	// Check if permissions match (masking out type bits)
	if mode.Perm() != expectedMode {
		t.Errorf("Expected file permissions %o, got %o", expectedMode, mode.Perm())
	}

	// Test directory permissions (0755)
	testDir := filepath.Join(tempDir, "testdir")
	err = writer.CreateDirectory(testDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	dirInfo, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	dirMode := dirInfo.Mode()
	expectedDirMode := os.FileMode(0755)

	if dirMode.Perm() != expectedDirMode {
		t.Errorf("Expected directory permissions %o, got %o", expectedDirMode, dirMode.Perm())
	}
}
