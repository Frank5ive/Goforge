package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

// errReported is returned by a command that already printed why it failed, so
// execute only sets the exit code.
var errReported = errors.New("reported")

// checkOptions holds the flag values for the check command.
type checkOptions struct {
	dir  string
	race bool
	json bool
}

func newCheckCmd() *cobra.Command {
	opts := &checkOptions{}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run gofmt, go vet, the tests, and golangci-lint, and report every problem",
		Long: `Check a Go project the way CI does, and report each problem with its file,
line, and message instead of the tools' raw output:

  format   gofmt
  vet      go vet (compile errors included)
  test     go test (each failing test, with where it failed)
  lint     golangci-lint, if it is installed

Tests and lint are skipped while the code does not compile. The command exits
with status 1 if any check fails. With --json, the report is printed as JSON
for scripts and AI agents.`,
		Example: `  goforge check
  goforge check --race
  goforge check --dir ./shop --json`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runCheck(cmd.OutOrStdout(), opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.dir, "dir", "d", ".", "Project directory (the one containing go.mod)")
	flags.BoolVar(&opts.race, "race", false, "Run the tests with the race detector")
	flags.BoolVar(&opts.json, "json", false, "Print the report as JSON, for scripts and AI agents")

	return cmd
}

// checkResult is the JSON output of check.
type checkResult struct {
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Project string `json:"project"`
	domain.CheckReport
}

func runCheck(out io.Writer, opts *checkOptions) error {
	container, err := di.BuildContainer(templatesFS, di.Options{})
	if err != nil {
		return err
	}

	project := opts.dir
	if abs, err := filepath.Abs(project); err == nil {
		project = abs
	}
	if !opts.json {
		fmt.Fprintf(out, "🔍 Checking %s\n", project)
	}

	report, err := container.ProjectChecker.Check(opts.dir, domain.CheckOptions{Race: opts.race})
	if err != nil {
		return err
	}

	if opts.json {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(checkResult{Command: "check", OK: report.Passed(), Project: project, CheckReport: report}); err != nil {
			return err
		}
	} else {
		printCheckReport(out, report)
	}

	if !report.Passed() {
		return errReported
	}
	return nil
}

func printCheckReport(out io.Writer, report domain.CheckReport) {
	for _, step := range report.Steps {
		switch step.Status {
		case domain.StepPassed:
			fmt.Fprintf(out, "   ✓ %-7s %s\n", step.Name, duration(step.DurationMS))
		case domain.StepFailed:
			fmt.Fprintf(out, "   ✗ %-7s %s  %d %s\n", step.Name, duration(step.DurationMS), step.Problems, plural(step.Problems, "problem"))
		default:
			fmt.Fprintf(out, "   - %-7s skipped: %s\n", step.Name, step.Note)
		}
	}

	for _, p := range report.Problems {
		location := p.File
		if p.Line > 0 {
			location = fmt.Sprintf("%s:%d", p.File, p.Line)
		}
		label := p.Step
		switch {
		case p.Test != "":
			label = p.Test
		case p.Linter != "":
			label = "lint (" + p.Linter + ")"
		}
		fmt.Fprintf(out, "\n❌ %s  %s\n", strings.TrimSpace(location), label)
		for line := range strings.Lines(p.Message) {
			fmt.Fprintf(out, "   %s", line)
		}
		fmt.Fprintln(out)
	}

	if report.Passed() {
		fmt.Fprintln(out, "\n✅ All checks passed.")
		return
	}
	them := "them"
	if len(report.Problems) == 1 {
		them = "it"
	}
	fmt.Fprintf(out, "\n%d %s. Fix %s and run `goforge check` again.\n",
		len(report.Problems), plural(len(report.Problems), "problem"), them)
}

func duration(ms int64) string {
	return (time.Duration(ms) * time.Millisecond).Round(100 * time.Millisecond).String()
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
