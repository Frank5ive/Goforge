package main

import (
	"bufio"
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
	scratch = "scratch" // the agent writes everything itself
	goforge = "goforge" // GoForge is installed, and the prompt says so
)

// runConfig configures one agent run.
type runConfig struct {
	task, condition string
	run             int
	model           string
	budgetUSD       float64
	timeout         time.Duration
	goforgeBin      string // directory with the goforge binary, for the goforge condition
	workRoot        string // where run directories are created
}

// Result is the outcome of one agent run.
type Result struct {
	Task      string    `json:"task"`
	Condition string    `json:"condition"`
	Run       int       `json:"run"`
	Model     string    `json:"model"`
	Started   time.Time `json:"started"`
	Agent     Agent     `json:"agent"`
	Score     Score     `json:"score"`
	WorkDir   string    `json:"work_dir"`
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
	GoforgeCommands  int `json:"goforge_commands"`
	// PermissionDenials counts tool calls the allow-list refused.
	PermissionDenials int `json:"permission_denials"`
}

// allowedTools is the agent's tool allow-list, the same in both conditions.
// Shell commands are limited to the Go toolchain and common file and process
// tools; nothing outside the run directory needs anything else.
var allowedTools = []string{
	"Read", "Edit", "Write", "Glob", "Grep", "TodoWrite",
	"Bash(go:*)", "Bash(gofmt:*)", "Bash(goforge:*)", "Bash(make:*)",
	"Bash(curl:*)", "Bash(ls:*)", "Bash(cat:*)", "Bash(head:*)", "Bash(tail:*)", "Bash(wc:*)",
	"Bash(grep:*)", "Bash(find:*)", "Bash(sed:*)", "Bash(echo:*)", "Bash(pwd)", "Bash(which:*)",
	"Bash(mkdir:*)", "Bash(cp:*)", "Bash(mv:*)", "Bash(touch:*)",
	"Bash(sleep:*)", "Bash(kill:*)", "Bash(pkill:*)", "Bash(ps:*)", "Bash(timeout:*)",
	"Bash(sqlite3:*)", "Bash(psql:*)",
}

// disallowedTools keeps runs comparable: no web lookups and no subagents.
var disallowedTools = []string{"WebFetch", "WebSearch", "Task", "Agent"}

// runAgent runs the agent on the task in a fresh directory, then scores the
// service it built.
func runAgent(cfg runConfig) (Result, error) {
	spec, err := os.ReadFile(filepath.Join("tasks", cfg.task, "spec.md"))
	if err != nil {
		return Result{}, fmt.Errorf("reading the task spec: %w", err)
	}

	started := time.Now()
	dir := filepath.Join(cfg.workRoot, fmt.Sprintf("%s-%s-%d-%s", cfg.task, cfg.condition, cfg.run, started.Format("20060102-150405")))
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return Result{}, err
	}

	var devDatabase string
	if cfg.task == "orders" {
		url, drop, err := freshPostgresDatabase()
		if err != nil {
			return Result{}, err
		}
		defer drop()
		devDatabase = url
	}

	prompt := buildPrompt(cfg.condition, string(spec), devDatabase)
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		return Result{}, err
	}

	env, err := agentEnv(cfg)
	if err != nil {
		return Result{}, err
	}

	args := []string{
		"-p", prompt,
		"--output-format", "stream-json", "--verbose",
		"--model", cfg.model,
		"--max-budget-usd", fmt.Sprintf("%.2f", cfg.budgetUSD),
		"--permission-mode", "dontAsk",
		"--allowedTools", strings.Join(allowedTools, " "),
		"--disallowedTools", strings.Join(disallowedTools, " "),
		// No MCP servers, plugins, hooks, or user settings: only what's above
		"--strict-mcp-config",
		"--setting-sources", "project",
		"--no-session-persistence",
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = work
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = transcript, stderr
	runErr := cmd.Run()

	agent, err := parseTranscript(filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		return Result{}, fmt.Errorf("reading the transcript: %w", err)
	}
	if ctx.Err() != nil {
		agent.Outcome = "timeout"
	} else if runErr != nil && agent.Outcome == "" {
		agent.Outcome = "error: " + runErr.Error()
	}

	return Result{
		Task: cfg.task, Condition: cfg.condition, Run: cfg.run, Model: cfg.model, Started: started,
		Agent: agent, Score: score(cfg.task, filepath.Join(work, "service")), WorkDir: dir,
	}, nil
}

// buildPrompt returns the task prompt. The conditions differ only in the
// sentence about GoForge.
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
	if condition == goforge {
		b.WriteString("\nGoForge (`goforge`), a CLI that generates and extends Go REST API projects, is installed. Run `goforge --help` to see what it can do.\n")
	}
	b.WriteString("\n---\n\n")
	b.WriteString(spec)
	return b.String()
}

// agentEnv returns the agent's environment: this process's, without Claude
// Code's own variables (so the agent doesn't think it is nested in a session),
// and with goforge on PATH only in the goforge condition.
func agentEnv(cfg runConfig) ([]string, error) {
	var env []string
	var path string
	for _, kv := range os.Environ() {
		key, value, _ := strings.Cut(kv, "=")
		switch {
		case key == "CLAUDECODE" || strings.HasPrefix(key, "CLAUDE_CODE_"):
			continue
		case key == "PATH":
			path = value
			continue
		}
		env = append(env, kv)
	}

	var dirs []string
	if cfg.condition == goforge {
		dirs = append(dirs, cfg.goforgeBin)
	}
	for _, dir := range filepath.SplitList(path) {
		if _, err := os.Stat(filepath.Join(dir, "goforge")); err == nil {
			continue // keep any other goforge out of both conditions
		}
		dirs = append(dirs, dir)
	}
	return append(env, "PATH="+strings.Join(dirs, string(os.PathListSeparator))), nil
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
