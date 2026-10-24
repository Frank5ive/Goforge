package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Frank5ive/goforge/internal/di"
	"github.com/Frank5ive/goforge/internal/domain"
)

// inspectOptions holds the flag values for the inspect command.
type inspectOptions struct {
	dir  string
	json bool
}

func newInspectCmd() *cobra.Command {
	opts := &inspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Describe a project: its resources, routes, database, and settings",
		Long: `Describe a project created by goforge generate without reading its code: its
module and Go version, features, database, resources and their fields, every
route it serves, the environment variables it reads, and the marker comments
` + "`goforge add`" + ` edits files at.

It also reports problems that would stop a ` + "`goforge add`" + ` command from working,
such as a marker comment that was deleted. With --json, the description is
printed as JSON for scripts and AI agents.`,
		Example: `  goforge inspect
  goforge inspect --dir ./shop --json`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runInspect(cmd.OutOrStdout(), opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.dir, "dir", "d", ".", "Project directory (the one containing go.mod)")
	flags.BoolVar(&opts.json, "json", false, "Print the description as JSON, for scripts and AI agents")

	return cmd
}

// inspectResult is the JSON output of inspect: the project description plus the
// fields every command's result has.
type inspectResult struct {
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Project string `json:"project"`
	domain.ProjectInfo
}

func runInspect(out io.Writer, opts *inspectOptions) error {
	container, err := di.BuildContainer(templatesFS, di.Options{})
	if err != nil {
		return err
	}
	info, err := container.ProjectInspector.Inspect(opts.dir)
	if err != nil {
		return err
	}

	project := opts.dir
	if abs, err := filepath.Abs(project); err == nil {
		project = abs
	}

	if opts.json {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(inspectResult{Command: "inspect", OK: true, Project: project, ProjectInfo: info})
	}
	printProjectInfo(out, project, info)
	return nil
}

func printProjectInfo(out io.Writer, project string, info domain.ProjectInfo) {
	fmt.Fprintf(out, "🔎 %s · %s · Go %s\n", info.AppName, info.Module, info.GoVersion)
	fmt.Fprintf(out, "   %s\n", project)

	database := "none (in memory)"
	if info.Database != "" {
		database = domain.DatabaseLabel(info.Database)
	}
	var features []string
	for _, f := range []struct {
		on   bool
		name string
	}{
		{info.Features.Logger, "request logging"}, {info.Features.Docker, "Dockerfile"},
		{info.Features.Readme, "README"}, {info.Features.CI, "GitHub Actions CI"},
	} {
		if f.on {
			features = append(features, f.name)
		}
	}
	if len(features) == 0 {
		features = []string{"none"}
	}
	fmt.Fprintf(out, "   Port %d · Database: %s · Features: %s\n", info.Port, database, strings.Join(features, ", "))

	fmt.Fprintf(out, "\n📦 Resources (%d):\n", len(info.Resources))
	if len(info.Resources) == 0 {
		fmt.Fprintln(out, "   none yet; add one with: goforge add resource <name> --fields \"name:string\"")
	}
	for _, r := range info.Resources {
		fields := make([]string, 0, len(r.Fields))
		for _, f := range r.Fields {
			fields = append(fields, f.Spec())
		}
		fmt.Fprintf(out, "   %-12s %-16s %-12s %s\n", r.Name, r.Path, r.Package+"/", strings.Join(fields, ", "))
	}

	fmt.Fprintf(out, "\n🌐 Routes (%d):\n", len(info.Routes))
	for _, route := range info.Routes {
		fmt.Fprintf(out, "   %-6s %s\n", route.Method, route.Path)
	}

	fmt.Fprintf(out, "\n⚙️  Environment: %s\n", strings.Join(info.Env, ", "))

	present := 0
	for _, m := range info.Markers {
		if m.Present {
			present++
		}
	}
	fmt.Fprintf(out, "🧩 Markers: %d of %d present\n", present, len(info.Markers))

	if len(info.Problems) == 0 {
		fmt.Fprintln(out, "\n✅ No problems: `goforge add` commands can extend this project.")
		return
	}
	fmt.Fprintln(out)
	for _, problem := range info.Problems {
		fmt.Fprintf(out, "⚠️  %s\n", problem)
	}
}
