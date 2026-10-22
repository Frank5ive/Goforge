package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

var templatesFS fs.FS

// SetTemplatesFS sets the filesystem that holds the *.tmpl files at its root.
func SetTemplatesFS(fsys fs.FS) {
	templatesFS = fsys
}

// errAborted is returned when the user cancels the interactive prompts.
var errAborted = errors.New("generation cancelled")

// generateOptions holds the flag values for the generate command.
type generateOptions struct {
	appName    string
	modulePath string
	port       int
	outputDir  string

	logger bool
	docker bool
	readme bool
	ci     bool

	yes     bool
	dryRun  bool
	initGit bool
}

// feature is an optional part of a generated project that can be toggled by flag or prompt.
type feature struct {
	flag  string
	label string
	value func(opts *generateOptions) *bool
}

var features = []feature{
	{flag: "logger", label: "Request logging", value: func(o *generateOptions) *bool { return &o.logger }},
	{flag: "docker", label: "Dockerfile", value: func(o *generateOptions) *bool { return &o.docker }},
	{flag: "readme", label: "README.md", value: func(o *generateOptions) *bool { return &o.readme }},
	{flag: "ci", label: "GitHub Actions CI", value: func(o *generateOptions) *bool { return &o.ci }},
}

func newGenerateCmd() *cobra.Command {
	opts := &generateOptions{}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new Go HTTP server",
		Long: `Generate a production-ready Go HTTP server.

The generated server will include:
  - Health check endpoint
  - Graceful shutdown
  - Configurable port (flag default, PORT environment variable at runtime)
  - Panic recovery middleware and HTTP timeouts
  - Request ID and panic recovery middleware
  - Tests, a Makefile, and a golangci-lint config
  - Optional request logging, Dockerfile, README, and GitHub Actions CI

Features not set by flag are asked for interactively, unless --yes is given.`,
		Example: `  goforge generate --name=my-api --module=github.com/acme/my-api
  goforge generate -n user-service -p 3000 -o ./services --yes
  goforge generate -n worker --docker=false --yes --git
  goforge generate -n my-api --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Flag errors above print usage; generation errors below do not
			cmd.SilenceUsage = true
			return runGenerate(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.appName, "name", "n", "myapp", "Name of the application (also the project directory)")
	flags.StringVarP(&opts.modulePath, "module", "m", "", "Go module path, e.g. github.com/acme/my-api (default: the app name)")
	flags.IntVarP(&opts.port, "port", "p", 8080, "Default port for the server (PORT overrides it at runtime)")
	flags.StringVarP(&opts.outputDir, "output", "o", ".", "Directory to create the project in")
	flags.BoolVar(&opts.logger, "logger", true, "Include request logging")
	flags.BoolVar(&opts.docker, "docker", true, "Include a Dockerfile")
	flags.BoolVar(&opts.readme, "readme", true, "Include a README.md")
	flags.BoolVar(&opts.ci, "ci", true, "Include a GitHub Actions CI workflow")
	flags.BoolVarP(&opts.yes, "yes", "y", false, "Skip prompts and use flag values and defaults")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Show the files that would be created without writing anything")
	flags.BoolVar(&opts.initGit, "git", false, "Initialize a git repository in the new project")

	return cmd
}

func runGenerate(cmd *cobra.Command, opts *generateOptions) error {
	out := cmd.OutOrStdout()

	printBanner(out)

	if !opts.yes {
		if err := promptForOptions(cmd, opts); err != nil {
			return err
		}
	}

	config := &domain.ServerConfig{
		AppName:       opts.appName,
		ModulePath:    opts.modulePath,
		Port:          opts.port,
		OutputDir:     opts.outputDir,
		IncludeLogger: opts.logger,
		IncludeDocker: opts.docker,
		IncludeReadme: opts.readme,
		IncludeCI:     opts.ci,
	}

	displayConfig(out, config)

	container, err := di.BuildContainer(templatesFS, di.Options{DryRun: opts.dryRun})
	if err != nil {
		return err
	}

	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		return err
	}

	projectPath := config.GetProjectPath()
	files := container.Files.Files()

	if opts.dryRun {
		fmt.Fprintf(out, "🔍 Dry run: no files were written. GoForge would create %d files in %s:\n", len(files), projectPath)
		printFiles(out, projectPath, files)
		return nil
	}

	fmt.Fprintf(out, "✅ Created %d files in %s:\n", len(files), projectPath)
	printFiles(out, projectPath, files)

	if opts.initGit {
		initGitRepo(out, projectPath)
	}

	fmt.Fprintf(out, "\n📝 Next steps:\n")
	fmt.Fprintf(out, "   cd %s\n", projectPath)
	fmt.Fprintf(out, "   go run .\n")
	fmt.Fprintf(out, "\n🌐 Your server will be available at: http://localhost:%d\n", config.Port)
	fmt.Fprintf(out, "💚 Health endpoint: http://localhost:%d/health\n", config.Port)

	return nil
}

// errNotInteractive is returned when prompts are needed but stdin is not a terminal.
var errNotInteractive = errors.New("interactive prompts need a terminal; use --yes and set options with flags (--module, --logger, --docker, --readme, --ci)")

// promptForOptions asks for the module path and features that were not set by flag.
func promptForOptions(cmd *cobra.Command, opts *generateOptions) error {
	var fields []huh.Field

	if !cmd.Flags().Changed("module") {
		fields = append(fields, huh.NewInput().
			Title("Go module path").
			Description(fmt.Sprintf("e.g. github.com/you/%s (leave empty to use %q)", opts.appName, opts.appName)).
			Value(&opts.modulePath).
			Validate(func(path string) error {
				if path == "" {
					return nil
				}
				return domain.ValidateModulePath(path)
			}))
	}

	var promptable []feature
	var selected []string
	for _, f := range features {
		if cmd.Flags().Changed(f.flag) {
			continue
		}
		promptable = append(promptable, f)
		if *f.value(opts) {
			selected = append(selected, f.flag)
		}
	}

	if len(promptable) > 0 {
		options := make([]huh.Option[string], 0, len(promptable))
		for _, f := range promptable {
			options = append(options, huh.NewOption(f.label, f.flag).Selected(*f.value(opts)))
		}
		fields = append(fields, huh.NewMultiSelect[string]().
			Title("Features").
			Description("space or x to toggle, enter to confirm").
			Options(options...).
			Value(&selected))
	}

	if len(fields) == 0 {
		return nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errNotInteractive
	}

	form := huh.NewForm(huh.NewGroup(fields...))

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errAborted
		}
		return fmt.Errorf("failed to read answers: %w", err)
	}

	for _, f := range promptable {
		*f.value(opts) = slices.Contains(selected, f.flag)
	}

	return nil
}

func displayConfig(out io.Writer, config *domain.ServerConfig) {
	fmt.Fprintln(out, "📋 Configuration Summary:")
	fmt.Fprintf(out, "   • Application: %s\n", config.AppName)
	fmt.Fprintf(out, "   • Module: %s\n", config.GetModuleName())
	fmt.Fprintf(out, "   • Port: %d\n", config.Port)
	fmt.Fprintf(out, "   • Output: %s\n", config.GetProjectPath())
	fmt.Fprintf(out, "   • Request logging: %s\n", getBoolIcon(config.IncludeLogger))
	fmt.Fprintf(out, "   • Dockerfile: %s\n", getBoolIcon(config.IncludeDocker))
	fmt.Fprintf(out, "   • README: %s\n", getBoolIcon(config.IncludeReadme))
	fmt.Fprintf(out, "   • GitHub Actions CI: %s\n", getBoolIcon(config.IncludeCI))
	fmt.Fprintln(out)
}

// printFiles lists generated files relative to the project directory.
func printFiles(out io.Writer, projectPath string, files []string) {
	for _, file := range files {
		relative, err := filepath.Rel(projectPath, file)
		if err != nil {
			relative = file
		}
		fmt.Fprintf(out, "   %s\n", filepath.ToSlash(relative))
	}
}

// initGitRepo runs `git init` in the project. Failures are reported but are not
// fatal, since the project itself was generated successfully.
func initGitRepo(out io.Writer, projectPath string) {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(out, "⚠️  git not found in PATH; skipping repository initialization")
		return
	}

	gitInit := exec.Command("git", "init", "--quiet")
	gitInit.Dir = projectPath
	if output, err := gitInit.CombinedOutput(); err != nil {
		fmt.Fprintf(out, "⚠️  git init failed: %v\n%s", err, output)
		return
	}

	fmt.Fprintln(out, "🌱 Initialized a git repository")
}

func getBoolIcon(value bool) string {
	if value {
		return "✓"
	}
	return "✗"
}
