package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountLinesSeparatesCodeFromTests(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":           "package main\n\nfunc main() {}\n",  // 3
		"util/util.go":      "package util\n",                    // 1
		"util/util_test.go": "package util\n\nfunc TestX() {}\n", // 3
		"README.md":         "not code\nnot code\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if code, tests := countLines(dir); code != 4 || tests != 3 {
		t.Errorf("countLines() = %d code, %d test lines; want 4 and 3", code, tests)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("  short \n"); got != "short" {
		t.Errorf("truncate() = %q, want it trimmed", got)
	}
	long := truncate(strings.Repeat("x", 2000))
	if len(long) > 1600 || !strings.HasSuffix(long, "(truncated)") {
		t.Errorf("truncate() of a long string = %d characters, want it cut and marked", len(long))
	}
}
