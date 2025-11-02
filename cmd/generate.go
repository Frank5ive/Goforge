package cmd

import (
	"embed"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

var templatesFS embed.FS

// SetTemplatesFS sets the embedded templates filesystem
func SetTemplatesFS(fs embed.FS) {
	templatesFS = fs
}

var (
	appName       string
	port          int
	outputDir     string
	nonInteractive bool
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new Go HTTP server",
	Long: `Generate a production-ready Go HTTP server with clean architecture.

The generated server will include:
  - Health check endpoint
  - Graceful shutdown
  - Configurable port
  - Clean architecture structure
  - Optional logging and middleware`,
	Example: `  minicli1 generate --name=myapp --port=8080
  minicli1 generate -n userservice -p 3000`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGenerate(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	generateCmd.Flags().StringVarP(&appName, "name", "n", "myapp", "Name of the application")
	generateCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port number for the server")
	generateCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for generated project")
	generateCmd.Flags().BoolVar(&nonInteractive, "yes", false, "Skip interactive prompts and use defaults")
	
	rootCmd.AddCommand(generateCmd)
}

func runGenerate() error {
	fmt.Println("🎯 Starting server generation...")
	fmt.Printf("📦 Project name: %s\n", appName)
	fmt.Printf("🔌 Port: %d\n", port)
	
	// Interactive feature selection
	config, err := promptForFeatures()
	if err != nil {
		return fmt.Errorf("failed to get user preferences: %w", err)
	}
	
	// Display configuration
	displayConfig(config)
	
	// Build DI container with embedded templates
	fmt.Println("🔧 Initializing dependencies...")
	container := di.BuildContainerWithEmbed(templatesFS, "templates")

	// Call usecase to generate server
	fmt.Println("🚀 Generating server project...")
	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// Success message
	fmt.Println("\n✅ Server generation complete!")
	fmt.Printf("\n📝 Next steps:\n")
	fmt.Printf("   cd %s\n", config.AppName)
	fmt.Printf("   go mod download\n")
	fmt.Printf("   go run main.go\n")
	fmt.Printf("\n🌐 Your server will be available at: http://localhost:%d\n", config.Port)
	fmt.Printf("💚 Health endpoint: http://localhost:%d/health\n", config.Port)
	
	return nil
}

func promptForFeatures() (*domain.ServerConfig, error) {
	config := &domain.ServerConfig{
		AppName:   appName,
		Port:      port,
		OutputDir: outputDir,
	}
	
	// Use defaults if non-interactive mode
	if nonInteractive {
		config.IncludeLogger = true
		config.IncludeDocker = true
		config.IncludeReadme = true
		return config, nil
	}
	
	// Logger preference
	loggerPrompt := &survey.Confirm{
		Message: "Include structured logger?",
		Default: true,
		Help:    "Adds a simple structured logger for HTTP requests and server events",
	}
	if err := survey.AskOne(loggerPrompt, &config.IncludeLogger); err != nil {
		return nil, err
	}
	
	// Docker preference
	dockerPrompt := &survey.Confirm{
		Message: "Generate Dockerfile?",
		Default: true,
		Help:    "Creates a multi-stage Dockerfile for containerized deployment",
	}
	if err := survey.AskOne(dockerPrompt, &config.IncludeDocker); err != nil {
		return nil, err
	}
	
	// README preference
	readmePrompt := &survey.Confirm{
		Message: "Generate README.md?",
		Default: true,
		Help:    "Creates a README with usage instructions and API documentation",
	}
	if err := survey.AskOne(readmePrompt, &config.IncludeReadme); err != nil {
		return nil, err
	}
	
	return config, nil
}

func displayConfig(config *domain.ServerConfig) {
	fmt.Println("\n📋 Configuration Summary:")
	fmt.Printf("   • Application: %s\n", config.AppName)
	fmt.Printf("   • Port: %d\n", config.Port)
	fmt.Printf("   • Output: %s\n", config.GetProjectPath())
	fmt.Printf("   • Logger: %v\n", getBoolIcon(config.IncludeLogger))
	fmt.Printf("   • Dockerfile: %v\n", getBoolIcon(config.IncludeDocker))
	fmt.Printf("   • README: %v\n", getBoolIcon(config.IncludeReadme))
	fmt.Println()
}

func getBoolIcon(value bool) string {
	if value {
		return "✓"
	}
	return "✗"
}
