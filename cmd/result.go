package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Frank5ive/goforge/internal/domain"
)

// result is what a command did. Every command builds one and prints it either as
// text for people or, with --json, as JSON for programs such as AI agents, so the
// two always report the same thing.
type result struct {
	// Command is the command that ran, e.g. "add resource".
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	DryRun  bool   `json:"dry_run"`
	// Project is the absolute path of the project directory.
	Project string `json:"project,omitempty"`
	Summary string `json:"summary,omitempty"`
	// Created and Updated are paths relative to Project, with forward slashes.
	Created   []string   `json:"created"`
	Updated   []string   `json:"updated"`
	Endpoints []endpoint `json:"endpoints,omitempty"`
	// Notes are follow-up steps the command could not do itself.
	Notes []string `json:"notes,omitempty"`
	// Info is useful context, such as where data is stored.
	Info      []string   `json:"info,omitempty"`
	NextSteps []nextStep `json:"next_steps,omitempty"`
	Error     *errorInfo `json:"error,omitempty"`
}

type endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// Query lists the optional query parameters, e.g. "limit=20&offset=0".
	Query string `json:"query,omitempty"`
}

type nextStep struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

type errorInfo struct {
	// Code is stable for programs to match on; see the domain.Code* constants.
	Code    string `json:"code"`
	Message string `json:"message"`
	// Hint suggests how to fix the problem.
	Hint string `json:"hint,omitempty"`
}

// newResult returns a successful, empty result for the command at project.
func newResult(command, project string, dryRun bool) *result {
	if abs, err := filepath.Abs(project); err == nil {
		project = abs
	}
	return &result{Command: command, OK: true, DryRun: dryRun, Project: project, Created: []string{}, Updated: []string{}}
}

// print writes r as JSON, as a compact summary when out is piped (as it is for
// scripts and AI agents, where every line costs reading time), or in full text
// for a terminal.
func (r *result) print(out io.Writer, asJSON bool) error {
	switch {
	case asJSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case piped(out):
		r.printCompact(out)
	default:
		r.printText(out)
	}
	return nil
}

// piped reports whether out is a pipe or file rather than a terminal. Other
// writers, such as test buffers, get the full text.
func piped(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && !term.IsTerminal(int(f.Fd()))
}

// printCompact writes the summary, counts instead of file lists, the routes on
// one line, notes, and next steps on one line. --json has the full details.
func (r *result) printCompact(out io.Writer) {
	prefix := "✅ "
	if r.DryRun {
		prefix = "🔍 Dry run: "
	}
	created, updated := "files created", "updated"
	if r.DryRun {
		created, updated = "files would be created", "would be updated"
	}
	fmt.Fprintf(out, "%s%s (%d %s, %d %s; --json lists them)\n",
		prefix, strings.TrimSuffix(r.Summary, "."), len(r.Created), created, len(r.Updated), updated)
	switch {
	case len(r.Endpoints) > 6:
		// Several resources: name their collections, not every route
		var collections []string
		for _, e := range r.Endpoints {
			if !strings.Contains(e.Path, "{") && !slices.Contains(collections, e.Path) {
				collections = append(collections, e.Path)
			}
		}
		fmt.Fprintf(out, "Routes: GET (paged), POST, and GET, PUT, PATCH, DELETE /{id} at %s\n", strings.Join(collections, ", "))
	case len(r.Endpoints) > 0:
		routes := make([]string, 0, len(r.Endpoints))
		for _, e := range r.Endpoints {
			routes = append(routes, e.Method+" "+e.Path)
		}
		fmt.Fprintf(out, "Routes: %s\n", strings.Join(routes, ", "))
	}
	for _, note := range r.Notes {
		fmt.Fprintf(out, "⚠️  %s\n", note)
	}
	if len(r.NextSteps) > 0 {
		steps := make([]string, 0, len(r.NextSteps))
		for _, step := range r.NextSteps {
			steps = append(steps, step.Command)
		}
		fmt.Fprintf(out, "Next: %s\n", strings.Join(steps, " · "))
	}
}

func (r *result) printText(out io.Writer) {
	if r.DryRun {
		fmt.Fprintf(out, "🔍 Dry run: no files were written. %s\n", r.Summary)
	} else {
		fmt.Fprintf(out, "✅ %s\n", r.Summary)
	}

	if len(r.Created) > 0 {
		if r.DryRun {
			fmt.Fprintf(out, "\nWould create %d files:\n", len(r.Created))
		} else {
			fmt.Fprintf(out, "\nCreated %d files:\n", len(r.Created))
		}
		printList(out, r.Created)
	}
	if len(r.Updated) > 0 {
		if r.DryRun {
			fmt.Fprintln(out, "\n…and would update:")
		} else {
			fmt.Fprintln(out, "\n📝 Updated:")
		}
		printList(out, r.Updated)
	}

	if len(r.Endpoints) > 0 {
		fmt.Fprintln(out, "\n🌐 Endpoints:")
		for _, e := range r.Endpoints {
			path := e.Path
			if e.Query != "" {
				path += "?" + e.Query
			}
			fmt.Fprintf(out, "   %-6s %s\n", e.Method, path)
		}
	}

	if len(r.Notes) > 0 {
		fmt.Fprintln(out)
		for _, note := range r.Notes {
			fmt.Fprintf(out, "⚠️  %s\n", note)
		}
	}
	if len(r.Info) > 0 {
		fmt.Fprintln(out)
		for _, info := range r.Info {
			fmt.Fprintf(out, "💡 %s\n", info)
		}
	}

	if len(r.NextSteps) > 0 {
		fmt.Fprintln(out, "\n📝 Next steps:")
		width := 0
		for _, step := range r.NextSteps {
			width = max(width, len(step.Command))
		}
		for _, step := range r.NextSteps {
			if step.Description == "" {
				fmt.Fprintf(out, "   %s\n", step.Command)
				continue
			}
			fmt.Fprintf(out, "   %-*s  # %s\n", width, step.Command, step.Description)
		}
	}
}

func printList(out io.Writer, paths []string) {
	for _, path := range paths {
		fmt.Fprintf(out, "   %s\n", path)
	}
}

// relativePaths returns paths relative to dir, with forward slashes.
func relativePaths(dir string, paths []string) []string {
	relative := make([]string, 0, len(paths))
	for _, path := range paths {
		if rel, err := filepath.Rel(dir, path); err == nil {
			path = rel
		}
		relative = append(relative, filepath.ToSlash(path))
	}
	return relative
}

// Error codes for failures outside the generators.
const (
	codeUsage     = "invalid_arguments"
	codeCancelled = "cancelled"
)

// errorResult describes err, from the command c, as a failed result.
func errorResult(c *cobra.Command, err error) *result {
	code := domain.ErrorCode(err)
	switch {
	case isUsageError(err):
		code = codeUsage
	case errors.Is(err, errAborted):
		code = codeCancelled
	}
	return &result{
		Command: commandName(c),
		Created: []string{},
		Updated: []string{},
		Error:   &errorInfo{Code: code, Message: err.Error(), Hint: hint(err, code)},
	}
}

// hint suggests a fix for err: its own hint if it has one, otherwise the one
// for its code.
func hint(err error, code string) string {
	if h := domain.ErrorHint(err); h != "" {
		return h
	}
	return hints[code]
}

// hints suggest a fix for each error code.
var hints = map[string]string{
	codeUsage:                   "Run the command with --help to see its arguments and flags.",
	domain.CodeInvalidInput:     "Run the command with --help to see the accepted values.",
	domain.CodeNotAProject:      "Run the command inside a project created by `goforge generate`, or point --dir at one.",
	domain.CodeAlreadyExists:    "Nothing was changed. Use a name or directory that isn't taken.",
	domain.CodeMarkerMissing:    "A file lost a `// goforge:` marker comment GoForge edits at. Restore it, or make this change by hand.",
	domain.CodeDependencyFailed: "Check your network connection and Go proxy settings (GOPROXY), then run the command again. Nothing was changed.",
	domain.CodeWriteFailed:      "Check that the directory exists and is writable.",
}

// commandName returns c's path without the program name, e.g. "add resource".
func commandName(c *cobra.Command) string {
	if c == nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimPrefix(c.CommandPath(), "goforge"), " ")
}

// usageError marks a mistake in how a command was called, such as an unknown
// flag or a missing argument.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }

func (e usageError) Unwrap() error { return e.err }

func isUsageError(err error) bool {
	var usage usageError
	return errors.As(err, &usage) || strings.HasPrefix(err.Error(), "unknown command")
}

// usageArgs marks errors from validate as usage errors.
func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if err := validate(c, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}
