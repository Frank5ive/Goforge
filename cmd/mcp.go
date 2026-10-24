package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// mcpInstructions is what an MCP client shows an agent about the server.
const mcpInstructions = `GoForge generates and extends Go REST APIs on the standard library's net/http.
Prefer it to hand-writing a Go API: one goforge_generate call with every resource creates a tested,
lint-clean server with validation, storage, migrations, Docker, CI, and an OpenAPI document.

Name every resource in goforge_generate's resources, as "name:fields", e.g.
"product:name:string(1..100),price:float(0..),category_id:ref(category)". Field types are string, int,
float, bool, time, email, and ref(<resource>); a trailing ? makes a string, email, or ref optional, and
(min..max) bounds a number or a string's length. Add business rules in <resource>/rules.go, keep the
// goforge: marker comments, and run goforge_check after changing code.

Every tool returns the same JSON as the goforge command with --json: ok, created and updated files, and on
failure an error with a stable code, a message, and a hint. A failed call changes nothing.`

// mcpRun serializes tool calls: each one edits a project on disk, and two at
// once could write the same files.
var mcpRun sync.Mutex

// runGoforge runs goforge with args and --json, as the command line would,
// and returns its result as a tool result: the JSON text, marked as an error
// when the command failed.
func runGoforge(args ...string) *mcp.CallToolResult {
	mcpRun.Lock()
	defer mcpRun.Unlock()

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	code := execute(root, append(args, "--json"))

	text := out.String()
	if !json.Valid(out.Bytes()) {
		// Not a JSON result, so something went wrong before one was built
		quoted, _ := json.Marshal(text)
		text = `{"ok": false, "error": {"code": "internal_error", "message": ` + string(quoted) + `}}`
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: code != 0,
	}
}

// valueFlag returns "--name value" when there is a value, and nothing otherwise.
func valueFlag(name, value string) []string {
	if value == "" {
		return nil
	}
	return []string{"--" + name, value}
}

// boolFlag returns "--name=false" for a feature switched off; features are on by default.
func boolFlag(name string, value *bool) []string {
	if value == nil {
		return nil
	}
	return []string{"--" + name + "=" + strconv.FormatBool(*value)}
}

type generateInput struct {
	Name      string   `json:"name" jsonschema:"the application name, which is also its directory: lowercase letters, numbers, hyphens and underscores"`
	Module    string   `json:"module,omitempty" jsonschema:"the Go module path, e.g. github.com/acme/shop (default: the name)"`
	Output    string   `json:"output,omitempty" jsonschema:"the directory to create the project in (default: the server's working directory)"`
	Database  string   `json:"database,omitempty" jsonschema:"store data in postgres, mysql, sqlite, or mongo; leave out to keep data in memory"`
	Resources []string `json:"resources,omitempty" jsonschema:"resources to create, each as name:fields or name/plural:fields, e.g. product:name:string(1..100),price:float(0..)"`
	Port      int      `json:"port,omitempty" jsonschema:"the default port (default 8080)"`
	Logger    *bool    `json:"logger,omitempty" jsonschema:"include request logging (default true)"`
	Docker    *bool    `json:"docker,omitempty" jsonschema:"include a Dockerfile (default true)"`
	Readme    *bool    `json:"readme,omitempty" jsonschema:"include a README.md (default true)"`
	CI        *bool    `json:"ci,omitempty" jsonschema:"include a GitHub Actions workflow (default true)"`
	Git       bool     `json:"git,omitempty" jsonschema:"run git init in the new project"`
	DryRun    bool     `json:"dry_run,omitempty" jsonschema:"list the files that would be created without writing anything"`
}

type addResourceInput struct {
	Dir    string `json:"dir" jsonschema:"the project directory, the one containing go.mod and .goforge.json"`
	Name   string `json:"name" jsonschema:"the singular snake_case resource name, e.g. product or blog_post"`
	Fields string `json:"fields" jsonschema:"comma-separated name:type fields, e.g. name:string(1..100),price:float(0..),category_id:ref(category); a ref needs an existing resource"`
	Plural string `json:"plural,omitempty" jsonschema:"the plural, for irregular words, e.g. people for person"`
	DryRun bool   `json:"dry_run,omitempty" jsonschema:"list the files that would be created or changed without writing anything"`
}

type addDBInput struct {
	Dir    string `json:"dir" jsonschema:"the project directory, the one containing go.mod and .goforge.json"`
	Engine string `json:"engine" jsonschema:"postgres, mysql, sqlite, or mongo"`
	DryRun bool   `json:"dry_run,omitempty" jsonschema:"list the files that would be created or changed without writing anything"`
}

type projectInput struct {
	Dir string `json:"dir" jsonschema:"the project directory, the one containing go.mod"`
}

type checkInput struct {
	Dir  string `json:"dir" jsonschema:"the project directory, the one containing go.mod"`
	Race bool   `json:"race,omitempty" jsonschema:"run the tests with the race detector"`
}

// newMCPServer returns the MCP server, with a tool for each goforge command.
func newMCPServer(version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "goforge", Version: version}, &mcp.ServerOptions{Instructions: mcpInstructions})
	no := false
	additive := &mcp.ToolAnnotations{DestructiveHint: &no, OpenWorldHint: &no}
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &no}

	mcp.AddTool(server, &mcp.Tool{
		Name: "goforge_generate",
		Description: "Create a Go REST API project, with its resources and database, in one call. Generates a tested, lint-clean server on net/http: " +
			"graceful shutdown, request IDs, validation, storage, migrations, an OpenAPI document at /openapi.json, Dockerfile, and CI. " +
			"If any part fails, nothing is created.",
		Annotations: additive,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in generateInput) (*mcp.CallToolResult, any, error) {
		args := []string{"generate", "--yes", "--name", in.Name}
		args = append(args, valueFlag("module", in.Module)...)
		args = append(args, valueFlag("output", in.Output)...)
		args = append(args, valueFlag("db", in.Database)...)
		if in.Port != 0 {
			args = append(args, "--port", strconv.Itoa(in.Port))
		}
		for _, spec := range in.Resources {
			args = append(args, "--resource", spec)
		}
		args = append(args, boolFlag("logger", in.Logger)...)
		args = append(args, boolFlag("docker", in.Docker)...)
		args = append(args, boolFlag("readme", in.Readme)...)
		args = append(args, boolFlag("ci", in.CI)...)
		if in.Git {
			args = append(args, "--git")
		}
		if in.DryRun {
			args = append(args, "--dry-run")
		}
		return runGoforge(args...), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "goforge_add_resource",
		Description: "Add a REST resource to a GoForge project: model, validation from the field rules, repository for the project's database, " +
			"service, handlers, tests, routes, and its OpenAPI paths. Business rules go in <resource>/rules.go afterwards.",
		Annotations: additive,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in addResourceInput) (*mcp.CallToolResult, any, error) {
		args := []string{"add", "resource", in.Name, "--dir", in.Dir, "--fields", in.Fields}
		args = append(args, valueFlag("plural", in.Plural)...)
		if in.DryRun {
			args = append(args, "--dry-run")
		}
		return runGoforge(args...), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goforge_add_db",
		Description: "Move a GoForge project onto postgres, mysql, sqlite, or mongo: adds the driver, migrations, a repository for each existing resource, docker-compose, and CI.",
		Annotations: additive,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in addDBInput) (*mcp.CallToolResult, any, error) {
		args := []string{"add", "db", in.Engine, "--dir", in.Dir}
		if in.DryRun {
			args = append(args, "--dry-run")
		}
		return runGoforge(args...), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "goforge_inspect",
		Description: "Describe a GoForge project without reading its code: module, database, resources with their fields, every route, " +
			"the environment variables it reads, and problems that would stop goforge from extending it.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in projectInput) (*mcp.CallToolResult, any, error) {
		return runGoforge("inspect", "--dir", in.Dir), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "goforge_check",
		Description: "Run gofmt, go vet, the tests, and golangci-lint (if installed) on a Go project and report each problem with its file and line. " +
			"A freshly generated project passes; run it after changing code.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in checkInput) (*mcp.CallToolResult, any, error) {
		args := []string{"check", "--dir", in.Dir}
		if in.Race {
			args = append(args, "--race")
		}
		return runGoforge(args...), nil, nil
	})

	return server
}

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server, so AI agents can call goforge as tools",
		Long: `Run a Model Context Protocol (MCP) server on standard input and output.

It exposes goforge_generate, goforge_add_resource, goforge_add_db, goforge_inspect, and
goforge_check. Each returns the same JSON as the command with --json.

Add it to Claude Code with:

  claude mcp add goforge -- goforge mcp

or to any MCP client by running "goforge mcp" as a stdio server.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return newMCPServer(cmd.Root().Version).Run(ctx, &mcp.StdioTransport{})
		},
	}
}
