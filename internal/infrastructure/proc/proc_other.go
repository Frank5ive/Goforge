//go:build !unix

package proc

import "os/exec"

// killTree leaves cancellation to exec.CommandContext, which kills the command
// itself; Windows has no process groups to signal this way.
func killTree(*exec.Cmd) {}
