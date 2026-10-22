// Package cmd implements the goforge command-line interface.
package cmd

import (
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

Generated code uses the standard library's net/http, is tested, and passes its
own lint config. generate and the add commands accept --dry-run to preview
their changes, and generate, add, inspect, and check accept --json for output
that programs and AI agents can read.

Example:
  goforge generate --name shop --module github.com/you/shop --db postgres --yes
  cd shop
  goforge add resource product --fields "name:string,price:float"`,
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
	return root
}

// Execute runs goforge with the command-line arguments and returns the
// process's exit code. It prints any error: as a JSON result on stdout with
// --json, otherwise as text on stderr.
func Execute() int {
	return execute(newRootCmd(), os.Args[1:])
}

func execute(root *cobra.Command, args []string) int {
	root.SetArgs(args)
	// Usage text would break JSON output; the error result has a hint instead
	root.SilenceUsage = wantsJSON(nil, args)
	c, err := root.ExecuteC()
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
