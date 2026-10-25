// Package cmd implements the goforge command-line interface.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is set at build time by GoReleaser with
// -ldflags "-X github.com/Frank5ive/goforge/cmd.version=<version>".
// When unset, it falls back to the module version recorded by `go install`.
var version = ""

// newRootCmd returns the goforge command with all its subcommands. Each run gets
// a fresh command tree, so flag values never carry over.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "goforge",
		Short: "⚒️ Generate production-ready Go REST APIs",
		Long: `⚒️ GoForge generates production-ready Go REST APIs and keeps extending them.

  goforge generate        create a server: graceful shutdown, structured logging,
                          request IDs, health checks, tests, CI, and Docker
  goforge add resource    add a REST resource with validation, pagination, and tests
  goforge add db          store data in PostgreSQL, MySQL, SQLite, or MongoDB
  goforge inspect         describe a project: resources, routes, database, settings
  goforge check           run gofmt, go vet, the tests, and golangci-lint, and report
                          each problem with its file and line
  goforge mcp             run an MCP server, so agents can call these as tools

A whole API in one command (each --resource is name:fields, or name/plural:fields):

  goforge generate --name shop --module github.com/you/shop --db postgres \
    --resource "category/categories:name:string,description:string?" \
    --resource "product:name:string(1..100),price:float(0..),category_id:ref(category)"

Field types: string, int, float, bool, time, email, and ref(<resource>). A trailing
? makes a string, email, or ref optional, and (min..max) bounds a number or a
string's length; either end may be left out. A ref must be the ID of an existing
item (else 422), and that item can't be deleted while referenced (409). Each
resource gets paged list, create, get, PUT, PATCH, and DELETE routes, validation
with a test per rule, and a rules.go for custom business rules.

Generated code uses the standard library's net/http, is tested, and passes its
own lint config. generate and add accept --dry-run; generate, add, inspect, and
check accept --json. Output is a compact summary when piped.`,
		Run: func(cmd *cobra.Command, _ []string) {
			out := cmd.OutOrStdout()
			printBanner(out)
			fmt.Fprintln(out, "🚀 Quick Start:")
			fmt.Fprintln(out, "   goforge generate --name=my-api")
			fmt.Fprintln(out)
			fmt.Fprintln(out, "📖 Documentation:")
			fmt.Fprintln(out, "   goforge --help")
			fmt.Fprintln(out, "   goforge generate --help")
			fmt.Fprintln(out)
			fmt.Fprintln(out, "⭐ GitHub: https://github.com/Frank5ive/goforge")
		},
	}
	root.Version = version

	// execute prints errors itself, as text or JSON
	root.SilenceErrors = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err}
	})
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newInspectCmd())
	root.AddCommand(newCheckCmd())
	root.AddCommand(newMCPCmd())
	return root
}

// Execute runs goforge with the command-line arguments and returns the
// process's exit code. It prints any error: as a JSON result on stdout with
// --json, otherwise as text on stderr.
func Execute() int {
	return execute(newRootCmd(), os.Args[1:])
}

// commandContext returns the context cmd was run with, which is cancelled when
// the caller (such as an MCP client) gives up on the command.
func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func execute(root *cobra.Command, args []string) int {
	return executeContext(context.Background(), root, args)
}

// executeContext is execute with a context the commands honor: cancelling it
// stops the external tools they run (go, git), and the command fails.
func executeContext(ctx context.Context, root *cobra.Command, args []string) int {
	root.SetArgs(args)
	// Usage text would break JSON output; the error result has a hint instead
	root.SilenceUsage = wantsJSON(nil, args)
	c, err := root.ExecuteContextC(ctx)
	if err == nil {
		return 0
	}
	if errors.Is(err, errReported) {
		return 1
	}

	r := errorResult(c, err)
	if wantsJSON(c, args) {
		_ = r.print(root.OutOrStdout(), true)
		return 1
	}
	stderr := root.ErrOrStderr()
	fmt.Fprintf(stderr, "❌ Error: %v\n", err)
	if r.Error.Hint != "" {
		fmt.Fprintf(stderr, "💡 %s\n", r.Error.Hint)
	}
	return 1
}

// wantsJSON reports whether --json was given, even if the command line did not
// parse far enough to set the flag.
func wantsJSON(c *cobra.Command, args []string) bool {
	if c != nil {
		if asJSON, err := c.Flags().GetBool("json"); err == nil && asJSON {
			return true
		}
	}
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}

func init() {
	if version == "" {
		version = versionFromBuildInfo()
	}
}

// versionFromBuildInfo returns the module version recorded in the binary, e.g.
// "0.1.0" for `go install github.com/Frank5ive/goforge@v0.1.0`, or "dev" for
// builds without version information.
func versionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}

// displayVersion formats the version for humans: "v0.1.0", or "dev" as-is.
func displayVersion() string {
	if version != "" && version[0] >= '0' && version[0] <= '9' {
		return "v" + version
	}
	return version
}
