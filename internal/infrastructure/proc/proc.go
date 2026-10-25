// Package proc makes the external commands goforge runs (go, git) stop
// completely when their context is cancelled.
package proc

import (
	"context"
	"os/exec"
	"time"
)

// killGrace is how long Wait lets a cancelled command's output pipes drain
// before giving up on them, in case a process outside the group still holds them.
const killGrace = 2 * time.Second

// Command returns the command name with args, run by ctx: cancelling the
// context kills the command and every process it started. `go test` runs the
// tests in a child process, and killing only `go` would leave them running.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = killGrace
	killTree(cmd)
	return cmd
}
