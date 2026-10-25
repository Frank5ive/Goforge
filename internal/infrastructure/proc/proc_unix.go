//go:build unix

package proc

import (
	"os/exec"
	"syscall"
)

// killTree starts cmd in its own process group and makes cancellation kill the
// whole group.
func killTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// A negative pid signals the process group
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
