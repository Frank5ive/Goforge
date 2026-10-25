package gocheck

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

func installedWithLint() map[string]bool {
	return map[string]bool{"go": true, "gofmt": true, "golangci-lint": true}
}

func TestCheck_ReportsLintProblems(t *testing.T) {
	issues := `{"Issues":[{"FromLinter":"errcheck","Text":"unchecked","Pos":{"Filename":"main.go","Line":4,"Column":2}}]}`
	tools := &fakeTools{installed: installedWithLint()}
	run := func(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
		if strings.HasSuffix(name, "golangci-lint") {
			return []byte(issues), nil, errors.New("exit status 1")
		}
		return tools.run(ctx, dir, name, args...)
	}

	report, err := (&Checker{run: run, lookPath: tools.lookPath}).Check(t.Context(), project(t), domain.CheckOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed() || len(report.Problems) != 1 || report.Problems[0].Linter != "errcheck" {
		t.Errorf("report = %+v, want the one errcheck problem", report)
	}
}

// A linter that can't run is a failed step with its own message, not a pass.
func TestCheck_LintFailureWithoutUsableOutputFailsTheStep(t *testing.T) {
	tools := &fakeTools{installed: installedWithLint()}
	tests := []struct {
		name   string
		stdout string
		stderr string
		err    error
		want   string
	}{
		{"exits with an error", "not json", "config is invalid", errors.New("exit status 3"), "config is invalid"},
		{"exits 0 with unreadable output", "not json", "", nil, "reading golangci-lint output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := func(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
				if strings.HasSuffix(name, "golangci-lint") {
					return []byte(tt.stdout), []byte(tt.stderr), tt.err
				}
				return tools.run(ctx, dir, name, args...)
			}

			report, err := (&Checker{run: run, lookPath: tools.lookPath}).Check(t.Context(), project(t), domain.CheckOptions{})
			if err != nil {
				t.Fatal(err)
			}
			lint := report.Steps[len(report.Steps)-1]
			if report.Passed() || lint.Name != domain.CheckLint || lint.Status != domain.StepFailed ||
				len(report.Problems) != 1 || !strings.Contains(report.Problems[0].Message, tt.want) {
				t.Errorf("steps = %+v, problems = %+v; want a failed lint step saying %q", report.Steps, report.Problems, tt.want)
			}
		})
	}
}

func TestTruncateCapsLongMessages(t *testing.T) {
	short := strings.Repeat("a", maxMessage)
	if got := truncate(short); got != short {
		t.Error("truncate() changed a message at the limit")
	}
	long := strings.Repeat("a", maxMessage+50)
	got := truncate(long)
	if len(got) >= len(long) || !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("truncate() = %d characters ending %q, want it shortened and marked", len(got), got[len(got)-12:])
	}
}

// A cancelled check returns the cancellation, not the killed tools' output
// presented as problems with the project, whether or not golangci-lint is installed.
func TestCheck_StopsWhenCancelled(t *testing.T) {
	for name, installed := range map[string]map[string]bool{
		"with golangci-lint":    installedWithLint(),
		"without golangci-lint": {"go": true, "gofmt": true},
	} {
		t.Run(name, func(t *testing.T) {
			tools := &fakeTools{installed: installed}
			ctx, cancel := context.WithCancel(t.Context())
			run := func(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
				if len(args) > 0 && args[0] == "test" {
					cancel() // the caller gives up while the tests run
					return nil, []byte("signal: killed"), errors.New("signal: killed")
				}
				return tools.run(ctx, dir, name, args...)
			}

			report, err := (&Checker{run: run, lookPath: tools.lookPath}).Check(ctx, project(t), domain.CheckOptions{})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Check() error = %v, want context.Canceled", err)
			}
			if len(report.Problems) != 0 {
				t.Errorf("problems = %+v, want none: a stopped tool says nothing about the project", report.Problems)
			}
			for _, step := range report.Steps {
				if step.Name == domain.CheckTest {
					t.Errorf("steps = %+v, want no test step for the stopped tests", report.Steps)
				}
			}
		})
	}
}
