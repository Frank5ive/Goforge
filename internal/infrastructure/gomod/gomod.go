// Package gomod manages a generated project's Go module dependencies with the
// go command.
package gomod

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Frank5ive/goforge/internal/domain"
)

// Manager implements domain.ModuleManager by running the go command in the
// project. In dry-run mode it changes nothing.
type Manager struct {
	dryRun bool
}

// New returns a Manager; dryRun makes Require a no-op.
func New(dryRun bool) *Manager {
	return &Manager{dryRun: dryRun}
}

// Require implements domain.ModuleManager.Require with `go get` and `go mod tidy`.
func (m *Manager) Require(dir, module, version string) error {
	if m.dryRun {
		return nil
	}
	if _, err := exec.LookPath("go"); err != nil {
		return domain.WithCode(domain.CodeDependencyFailed, fmt.Errorf("the go command is needed to add %s: %w", module, err))
	}

	for _, args := range [][]string{{"get", module + "@" + version}, {"mod", "tidy"}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if output, err := cmd.CombinedOutput(); err != nil {
			return domain.WithCode(domain.CodeDependencyFailed, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, output))
		}
	}
	return nil
}
