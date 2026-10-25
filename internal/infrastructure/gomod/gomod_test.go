package gomod

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Frank5ive/goforge/internal/domain"
)

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/shop\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRequireDryRunChangesNothing(t *testing.T) {
	dir := project(t)
	before, _ := os.ReadFile(filepath.Join(dir, "go.mod"))

	if err := New(true).Require(t.Context(), dir, "example.com/driver", "v1.0.0"); err != nil {
		t.Fatalf("Require() in dry-run mode error = %v", err)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "go.mod")); string(after) != string(before) {
		t.Errorf("a dry run changed go.mod:\n%s", after)
	}
}

// Cancelling the context stops the go command, so a hung download can't hold a
// caller (such as an MCP server) forever.
func TestRequireStopsWhenCancelled(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found in PATH")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan error, 1)
	go func() { done <- New(false).Require(ctx, project(t), "example.com/driver", "v1.0.0") }()

	select {
	case err := <-done:
		if err == nil || domain.ErrorCode(err) != domain.CodeDependencyFailed || !errors.Is(err, context.Canceled) && err.Error() == "" {
			t.Errorf("Require() with a cancelled context error = %v, want a dependency_failed error", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Require() kept running after its context was cancelled")
	}
}
