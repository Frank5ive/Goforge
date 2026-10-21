package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "goforge",
	Short: "⚒️ Forge production-ready Go servers in seconds",
	Long: `⚒️ GoForge - Forge production-ready Go HTTP servers in seconds

GoForge generates production-ready Go HTTP servers. Get health endpoints,
graceful shutdown, request logging, and Docker support out of the box.

Features:
  • Health check endpoint with JSON response
  • Graceful shutdown with signal handling
  • Port override via the PORT environment variable
  • Optional request logging
  • Optional Docker support
  • Single binary, zero configuration

Example:
  goforge generate --name=my-api --port=8080
  goforge generate --name=user-service --port=3000 --yes`,
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		printBanner(os.Stdout)
		fmt.Println("🚀 Quick Start:")
		fmt.Println("   goforge generate --name=my-api --port=8080")
		fmt.Println()
		fmt.Println("📖 Documentation:")
		fmt.Println("   goforge --help")
		fmt.Println("   goforge generate --help")
		fmt.Println()
		fmt.Println("⭐ GitHub: https://github.com/Frank5ive/goforge")
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	// main prints returned errors once, to stderr
	rootCmd.SilenceErrors = true
	rootCmd.AddCommand(newGenerateCmd())
}
