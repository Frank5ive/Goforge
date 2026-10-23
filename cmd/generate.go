package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strings"

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
	// db is the database engine from --db or the prompt; "" means none
	db string

	yes     bool
	dryRun  bool
	initGit bool
	json    bool
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

Features not set by flag are asked for interactively. With --yes or --json, or when
input is not a terminal (as for scripts and AI agents), the flag values and
defaults are used instead.`,
		Example: `  goforge generate --name=my-api --module=github.com/acme/my-api
  goforge generate -n user-service -p 3000 -o ./services --yes
  goforge generate -n worker --docker=false --yes --git
  goforge generate -n my-api --dry-run
  goforge generate -n shop --db postgres --json`,
		Args: usageArgs(cobra.NoArgs),
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
	flags.StringVar(&opts.db, "db", "", "Store data in a database: postgres, mysql, sqlite, or mongo (same as running `goforge add db <engine>` afterwards)")
	flags.BoolVarP(&opts.yes, "yes", "y", false, "Skip prompts and use flag values and defaults")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Show the files that would be created without writing anything")
	flags.BoolVar(&opts.initGit, "git", false, "Initialize a git repository in the new project")
	flags.BoolVar(&opts.json, "json", false, "Print the result as JSON, for scripts and AI agents (implies no prompts)")

	return cmd
}

func runGenerate(cmd *cobra.Command, opts *generateOptions) error {
	out := cmd.OutOrStdout()
	// The banner and summary are for people at a terminal
	compact := opts.json || piped(out)
	if !compact {
		printBanner(out)
	}

	opts.db = domain.NormalizeDatabase(opts.db)
	if opts.db != "" && !slices.Contains(domain.DatabaseEngines, opts.db) {
		return domain.WithCode(domain.CodeInvalidInput,
			fmt.Errorf("unsupported --db %q (supported: %s)", opts.db, strings.Join(domain.DatabaseEngines, ", ")))
	}

	// Prompt only when a person is at the terminal
	if !opts.yes && !opts.json && term.IsTerminal(int(os.Stdin.Fd())) {
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

	if !compact {
		displayConfig(out, config, opts)
	}

	container, err := di.BuildContainer(templatesFS, di.Options{DryRun: opts.dryRun})
	if err != nil {
		return err
	}

	if err := container.ServerGeneratorUsecase.Generate(config); err != nil {
		return err
	}

	projectPath := config.GetProjectPath()
	r := newResult("generate", projectPath, opts.dryRun)
	r.Created = relativePaths(projectPath, container.Files.Files())

	if opts.dryRun {
		r.Summary = fmt.Sprintf("GoForge would create the %s project in %s.", config.AppName, projectPath)
		if opts.db != "" {
			r.Info = append(r.Info, fmt.Sprintf("%s would then be added, as with `goforge add db %s`.", domain.DatabaseLabel(opts.db), opts.db))
		}
		return r.print(out, opts.json)
	}

	r.Summary = fmt.Sprintf("Created the %s project in %s", config.AppName, projectPath)
	if opts.db != "" {
		label := domain.DatabaseLabel(opts.db)
		changes, err := container.DatabaseGeneratorUsecase.Add(&domain.DatabaseConfig{ProjectDir: projectPath, Engine: opts.db})
		if err != nil {
			return fmt.Errorf("the project was created in %s, but adding %s failed: %w\nRun `goforge add db %s` inside it to try again", projectPath, label, err, opts.db)
		}
		r.Summary += ", storing data in " + label
		// Files the database step changed were all just created, so list them once
		for _, path := range changes.Created {
			if !slices.Contains(r.Created, path) {
				r.Created = append(r.Created, path)
			}
		}
		r.Notes = append(r.Notes, changes.Notes...)
	}

	if opts.initGit {
		if err := initGitRepo(projectPath); err != nil {
			r.Notes = append(r.Notes, fmt.Sprintf("The git repository was not initialized: %v", err))
		} else {
			r.Info = append(r.Info, "Initialized a git repository.")
		}
	}

	r.NextSteps = append(r.NextSteps, nextStep{Command: "cd " + projectPath})
	if opts.db != "" && opts.db != domain.DatabaseSQLite {
		r.NextSteps = append(r.NextSteps, nextStep{Command: "make db-up", Description: "start " + domain.DatabaseLabel(opts.db) + " in Docker"})
	}
	r.NextSteps = append(r.NextSteps,
		nextStep{Command: "make run", Description: "start the server in development mode"},
		nextStep{Command: `goforge add resource product --fields "name:string,price:float"`, Description: "add a REST resource"},
	)
	r.Info = append(r.Info,
		fmt.Sprintf("The server listens on http://localhost:%d, with a health check at /health.", config.Port))

	return r.print(out, opts.json)
}

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

	if !cmd.Flags().Changed("db") {
		options := []huh.Option[string]{huh.NewOption("None (keep data in memory)", "")}
		for _, engine := range domain.DatabaseEngines {
			label := domain.DatabaseLabel(engine)
			if engine == domain.DatabaseMySQL {
				label += " / MariaDB"
			}
			options = append(options, huh.NewOption(label, engine))
		}
		fields = append(fields, huh.NewSelect[string]().
			Title("Database").
			Description("you can also add one later with `goforge add db`").
			Options(options...).
			Value(&opts.db))
	}

	if len(fields) == 0 {
		return nil
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

func displayConfig(out io.Writer, config *domain.ServerConfig, opts *generateOptions) {
	fmt.Fprintln(out, "📋 Configuration Summary:")
	fmt.Fprintf(out, "   • Application: %s\n", config.AppName)
	fmt.Fprintf(out, "   • Module: %s\n", config.GetModuleName())
	fmt.Fprintf(out, "   • Port: %d\n", config.Port)
	fmt.Fprintf(out, "   • Output: %s\n", config.GetProjectPath())
	fmt.Fprintf(out, "   • Request logging: %s\n", getBoolIcon(config.IncludeLogger))
	fmt.Fprintf(out, "   • Dockerfile: %s\n", getBoolIcon(config.IncludeDocker))
	fmt.Fprintf(out, "   • README: %s\n", getBoolIcon(config.IncludeReadme))
	fmt.Fprintf(out, "   • GitHub Actions CI: %s\n", getBoolIcon(config.IncludeCI))
	database := "none (in memory)"
	if opts.db != "" {
		database = domain.DatabaseLabel(opts.db)
	}
	fmt.Fprintf(out, "   • Database: %s\n", database)
	fmt.Fprintln(out)
}

// initGitRepo runs `git init` in the project.
func initGitRepo(projectPath string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git not found in PATH")
	}

	gitInit := exec.Command("git", "init", "--quiet")
	gitInit.Dir = projectPath
	if output, err := gitInit.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w: %s", err, output)
	}
	return nil
}

func getBoolIcon(value bool) string {
	if value {
		return "✓"
	}
	return "✗"
}
