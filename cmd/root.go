package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

var rootCmd = &cobra.Command{
	Use:   "goforge",
	Short: "⚒️ Forge production-ready Go servers in seconds",
	Long: `⚒️ GoForge - Forge production-ready Go HTTP servers with Clean Architecture

GoForge generates production-ready Go HTTP servers following Clean Architecture 
principles. Get health endpoints, graceful shutdown, structured logging, and 
Docker support out of the box.

Features:
  • Clean Architecture (Domain, Usecase, Infrastructure)
  • Health check endpoint with JSON response
  • Graceful shutdown with signal handling
  • Optional structured logging
  • Optional Docker support
  • Single binary, zero configuration

Example:
  goforge generate --name=my-api --port=8080
  goforge generate --name=user-service --port=3000 --yes`,
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚒️  Welcome to GoForge - Forge Production-Ready Go Servers")
		fmt.Println()
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
}

