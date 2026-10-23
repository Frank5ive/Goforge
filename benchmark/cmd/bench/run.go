package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Conditions an agent runs a task under.
const (
	// scratch: the agent writes everything itself.
	scratch = "scratch"
	// goforge: GoForge is on PATH and the prompt says so; the agent decides
	// whether to use it. This measures whether agents adopt it.
	goforge = "goforge"
	// goforgeRequired: the prompt tells the agent to build the service with
	// GoForge. This measures GoForge's effect when it is used.
	goforgeRequired = "goforge-required"
)

// conditions lists every condition, in the order runs alternate through them.
var conditions = []string{scratch, goforge, goforgeRequired}

// runConfig configures one agent run.
type runConfig struct {
	task, condition string
	run             int
	model           string
	budgetUSD       float64
	timeout         time.Duration
	image, imageID  string // the benchmark image's tag and ID
	goforgeVersion  string // the GoForge version in the image
	benchLinux      string // bench built for Linux, for scoring in the image
	envFile         string // Docker env file with the agent's credentials
	workRoot        string // where run directories are created
}

// Result is the outcome of one agent run.
type Result struct {
	Task      string `json:"task"`
	Condition string `json:"condition"`
	Run       int    `json:"run"`
	Model     string `json:"model"`
	// GoforgeVersion is the GoForge version the agent could use; empty for
	// scratch, which doesn't depend on it.
	GoforgeVersion string    `json:"goforge_version,omitempty"`
	Image          string    `json:"image"`
	ImageID        string    `json:"image_id"`
	Started        time.Time `json:"started"`
	Agent          Agent     `json:"agent"`
	Score          Score     `json:"score"`
	WorkDir        string    `json:"work_dir"`
}

// Agent is what the agent's run cost and did, from its event stream.
type Agent struct {
	Outcome      string  `json:"outcome"` // the result event's subtype, e.g. "success"
	CostUSD      float64 `json:"cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
	Turns        int     `json:"turns"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read_tokens"`
	CacheWrite   int     `json:"cache_write_tokens"`
	ToolCalls    int     `json:"tool_calls"`
	// GoCommands counts go build/test/vet/run (and goforge check) commands, and
	// FailedGoCommands those that failed: each failure is a correction cycle.
	GoCommands       int `json:"go_commands"`
	FailedGoCommands int `json:"failed_go_commands"`
	// GoforgeCommands counts every goforge command, including --help;
	// GoforgeGenerated only those that generate code (generate and add), which
	// is what "used GoForge" means.
	GoforgeCommands  int `json:"goforge_commands"`
	GoforgeGenerated int `json:"goforge_generated"`
	// PermissionDenials counts refused tool calls; it should be 0.
	PermissionDenials int `json:"permission_denials"`
}

// disallowedTools keeps runs comparable: no web lookups and no subagents.
// Everything else is allowed; the container is the boundary.
var disallowedTools = []string{"WebFetch", "WebSearch", "Task", "Agent"}

// runAgent runs the agent on the task in a fresh container, then scores the
// service it built in another container from the same image.
//
// The run directory is mounted at /bench in both: the agent works in
// /bench/work, with /bench/home as its home.
func runAgent(cfg runConfig) (Result, error) {
	spec, err := os.ReadFile(filepath.Join("tasks", cfg.task, "spec.md"))
	if err != nil {
		return Result{}, fmt.Errorf("reading the task spec: %w", err)
	}

	started := time.Now()
	// Docker names allow only letters, digits, and _.- (no @)
	name := fmt.Sprintf("%s-%s-%d-%s", cfg.task, strings.ReplaceAll(label(cfg.condition, cfg.goforgeVersion), "@", "_"),
		cfg.run, started.Format("20060102-150405"))
	dir, err := filepath.Abs(filepath.Join(cfg.workRoot, name))
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "work"), 0o755); err != nil {
		return Result{}, err
	}

	var devDatabase string
	if cfg.task == "orders" || cfg.task == "store" {
		db, drop, err := createDatabase()
		if err != nil {
			return Result{}, err
		}
		defer drop()
		devDatabase = containerDatabaseURL(db)
	}

	prompt := buildPrompt(cfg.condition, string(spec), devDatabase)
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		return Result{}, err
	}

	transcript, err := os.Create(filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		return Result{}, err
	}
	defer transcript.Close()
	stderr, err := os.Create(filepath.Join(dir, "stderr.txt"))
	if err != nil {
		return Result{}, err
	}
	defer stderr.Close()

	container := "bench-" + name
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", container,
		"--network", benchNetwork, "--user", hostUser(),
		"-v", dir+":/bench", "-w", "/bench/work",
		"--env-file", cfg.envFile, "-e", "BENCH_CONDITION="+cfg.condition,
		cfg.image,
		"claude", "-p", prompt,
		// The container starts with an empty home and work directory, so there is
		// no CLAUDE.md, settings, plugins, or hooks; this rules out MCP servers too.
		// (--bare would also do this, but it accepts only API keys, not the
		// subscription token from `claude setup-token`.)
		"--strict-mcp-config",
		"--dangerously-skip-permissions",
		"--disallowedTools", strings.Join(disallowedTools, " "),
		"--output-format", "stream-json", "--verbose",
		"--model", cfg.model,
		"--max-budget-usd", fmt.Sprintf("%.2f", cfg.budgetUSD),
		"--no-session-persistence",
	)
	cmd.Stdout, cmd.Stderr = transcript, stderr
	cmd.Cancel = func() error { // a timeout kills the container, not just the docker client
		_, err := docker("kill", container)
		return err
	}
	runErr := cmd.Run()

	agent, err := parseTranscript(filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		return Result{}, fmt.Errorf("reading the transcript: %w", err)
	}
	if ctx.Err() != nil {
		agent.Outcome = "timeout"
	} else if runErr != nil && agent.Outcome == "" {
		// The agent never ran (for example Docker or the credentials failed):
		// stop, rather than record a run the agent didn't do
		if agent.Turns == 0 && agent.ToolCalls == 0 {
			return Result{}, fmt.Errorf("the agent didn't start: %v: %s", runErr, strings.TrimSpace(readFile(filepath.Join(dir, "stderr.txt"))))
		}
		agent.Outcome = "error: " + strings.TrimSpace(readFile(filepath.Join(dir, "stderr.txt")))
	}

	s, err := scoreInContainer(cfg, dir)
	if err != nil {
		return Result{}, err
	}
	version := cfg.goforgeVersion
	if cfg.condition == scratch {
		version = ""
	}
	return Result{
		Task: cfg.task, Condition: cfg.condition, Run: cfg.run, Model: cfg.model, GoforgeVersion: version,
		Image: cfg.image, ImageID: cfg.imageID, Started: started,
		Agent: agent, Score: s, WorkDir: dir,
	}, nil
}

// scoreInContainer scores the service in dir/work/service inside the
// benchmark image, with fresh storage.
func scoreInContainer(cfg runConfig, dir string) (Score, error) {
	args := []string{"run", "--rm", "--network", benchNetwork, "--user", hostUser(),
		"-v", dir + ":/bench", "-v", cfg.benchLinux + ":/opt/bench/bench:ro", "-w", "/bench",
		cfg.image, "/opt/bench/bench", "score", "-json", "-task", cfg.task, "-dir", "/bench/work/service"}
	switch cfg.task {
	case "products-sqlite":
		args = append(args, "-database-url", "/bench/score/data/products.db", "-test-database-url", "/bench/score/test/products.db")
	case "orders", "store":
		db, drop, err := createDatabase()
		if err != nil {
			return Score{}, err
		}
		defer drop()
		testDB, dropTest, err := createDatabase()
		if err != nil {
			return Score{}, err
		}
		defer dropTest()
		args = append(args, "-database-url", containerDatabaseURL(db), "-test-database-url", containerDatabaseURL(testDB))
	}

	out, err := exec.Command("docker", args...).Output()
	var s Score
	if jsonErr := json.Unmarshal(out, &s); jsonErr != nil {
		return Score{}, fmt.Errorf("scoring: %v: %s", err, bytes.TrimSpace(out))
	}
	return s, nil
}

// buildPrompt returns the task prompt. The conditions differ only in the last
// sentence of the requirements, about GoForge.
func buildPrompt(condition, spec, devDatabase string) string {
	var b strings.Builder
	b.WriteString("Build the service described below in Go.\n\n")
	b.WriteString("Create it as a Go module in a new directory named `service` inside the current directory, with the main package at the root of `service`.\n\n")
	b.WriteString("Requirements:\n")
	b.WriteString("- Listen on the port given by the PORT environment variable.\n")
	b.WriteString("- Write tests for your code. When you're done, `go build ./...`, `go vet ./...`, and `go test ./...` must pass in `service`.\n")
	if devDatabase != "" {
		b.WriteString("- For development and tests, a PostgreSQL database is available at " + devDatabase + "\n")
	}
	b.WriteString("- Work on your own: nobody will answer questions. Stop when the service is complete.\n")
	switch condition {
	case goforge:
		b.WriteString("\nGoForge (`goforge`), a CLI that generates and extends Go REST API projects, is installed. Run `goforge --help` to see what it can do.\n")
	case goforgeRequired:
		b.WriteString("\nBuild the service with GoForge (`goforge`), a CLI that generates and extends Go REST API projects. It is installed: run `goforge --help` to see what it can do.\n")
	}
	b.WriteString("\n---\n\n")
	b.WriteString(spec)
	return b.String()
}

// event is the part of a stream-json line the harness reads.
type event struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		Content []struct {
			Type      string          `json:"type"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
			ToolUseID string          `json:"tool_use_id"`
			IsError   bool            `json:"is_error"`
		} `json:"content"`
	} `json:"message"`
	// The final "result" event
	TotalCostUSD float64 `json:"total_cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	Usage        struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		CacheReadTokens     int `json:"cache_read_input_tokens"`
		CacheCreationTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	PermissionDenials []json.RawMessage `json:"permission_denials"`
}

// parseTranscript reads the agent's stream-json output.
func parseTranscript(path string) (Agent, error) {
	f, err := os.Open(path)
	if err != nil {
		return Agent{}, err
	}
	defer f.Close()

	var a Agent
	goCommand := map[string]bool{} // tool_use IDs of go commands
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scanner.Scan() {
		var e event
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		switch e.Type {
		case "assistant":
			for _, c := range e.Message.Content {
				if c.Type != "tool_use" {
					continue
				}
				a.ToolCalls++
				if c.Name != "Bash" {
					continue
				}
				var input struct{ Command string }
				_ = json.Unmarshal(c.Input, &input)
				if isGoCommand(input.Command) {
					a.GoCommands++
					goCommand[c.ID] = true
				}
				if strings.Contains(input.Command, "goforge") {
					a.GoforgeCommands++
				}
				if strings.Contains(input.Command, "goforge generate") || strings.Contains(input.Command, "goforge add") {
					a.GoforgeGenerated++
				}
			}
		case "user":
			for _, c := range e.Message.Content {
				if c.Type == "tool_result" && c.IsError && goCommand[c.ToolUseID] {
					a.FailedGoCommands++
				}
			}
		case "result":
			a.Outcome = e.Subtype
			a.CostUSD = e.TotalCostUSD
			a.DurationMS = e.DurationMS
			a.Turns = e.NumTurns
			a.InputTokens = e.Usage.InputTokens
			a.OutputTokens = e.Usage.OutputTokens
			a.CacheRead = e.Usage.CacheReadTokens
			a.CacheWrite = e.Usage.CacheCreationTokens
			a.PermissionDenials = len(e.PermissionDenials)
		}
	}
	return a, scanner.Err()
}

// isGoCommand reports whether a shell command builds, vets, tests, or runs Go
// code, or runs goforge check.
func isGoCommand(command string) bool {
	for _, c := range []string{"go build", "go vet", "go test", "go run", "goforge check"} {
		if strings.Contains(command, c) {
			return true
		}
	}
	return false
}

// label names a condition with the GoForge version it used, e.g.
// "goforge-required@v0.2.0"; scratch doesn't depend on the version.
func label(condition, version string) string {
	if condition == scratch || version == "" {
		return condition
	}
	return condition + "@" + version
}
