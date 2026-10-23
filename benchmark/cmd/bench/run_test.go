package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveAllReadOnlyModuleCache(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	mod := filepath.Join(home, "go", "pkg", "mod", "example.com", "m@v1.0.0")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "m.go"), []byte("package m\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	// Go makes module cache directories read-only
	for _, dir := range []string{mod, filepath.Dir(mod)} {
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
	}

	if err := removeAll(home); err != nil {
		t.Fatalf("removeAll() error = %v", err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Error("home still exists")
	}
}
