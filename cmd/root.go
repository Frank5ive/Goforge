// Package cmd implements the goforge command-line interface.
package cmd

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is set at build time by GoReleaser with
// -ldflags "-X github.com/Frank5ive/goforge/cmd.version=<version>".
// When unset, it falls back to the module version recorded by `go install`.
var version = ""

var rootCmd = &cobra.Command{
	Use:   "goforge",
	Short: "⚒️ Forge production-ready Go servers in seconds",
	Long: `⚒️ GoForge - Forge production-ready Go HTTP servers in seconds

GoForge generates production-ready Go HTTP servers that use only the standard
library, with tests, linting, and deployment tooling out of the box.

Features:
  • Health endpoint, graceful shutdown, and Go 1.22+ routing
  • Request IDs, panic recovery, and HTTP timeouts
  • Structured logging with log/slog (optional)
  • Tests, Makefile, and golangci-lint config
  • Non-root Dockerfile and GitHub Actions CI (optional)
  • Single binary, zero configuration

Example:
  goforge generate --name=my-api --module=github.com/you/my-api
  goforge generate --name=user-service --port=3000 --yes`,
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

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	if version == "" {
		version = versionFromBuildInfo()
	}
	rootCmd.Version = version

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	// main prints returned errors once, to stderr
	rootCmd.SilenceErrors = true
	rootCmd.AddCommand(newGenerateCmd())
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
