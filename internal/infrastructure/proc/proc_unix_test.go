//go:build unix

package proc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// alive reports whether a process with the pid exists.
func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// Cancelling must stop the processes the command started, not just the command:
// `go test` runs the tests in a child, which would otherwise outlive it.
func TestCommandKillsTheWholeProcessTree(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// The shell starts a grandchild that sleeps, records its pid, and waits for it
	cmd := Command(ctx, "sh", "-c", "sleep 60 & echo $! > "+pidFile+"; wait")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var grandchild int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if data, err := os.ReadFile(pidFile); err == nil && strings.TrimSpace(string(data)) != "" {
			grandchild, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			break
		}
	}
	if grandchild == 0 || !alive(grandchild) {
		t.Fatalf("the grandchild did not start (pid %d)", grandchild)
	}
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the command kept running after its context was cancelled")
	}

	// The kill is asynchronous: give the grandchild a moment to disappear
	for deadline := time.Now().Add(5 * time.Second); alive(grandchild) && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	if alive(grandchild) {
		t.Errorf("the grandchild (pid %d) survived the cancellation", grandchild)
	}
}

func TestCommandRunsNormallyWithoutCancellation(t *testing.T) {
	out, err := Command(t.Context(), "sh", "-c", "echo hello").Output()
	if err != nil || strings.TrimSpace(string(out)) != "hello" {
		t.Errorf("Output() = %q, %v; want hello", out, err)
	}
}
