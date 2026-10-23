// Command bench runs the GoForge benchmark: AI agents build the same Go
// services from scratch and with GoForge, in identical Docker containers, and
// the results are compared.
//
//	bench score  -task products -dir path/to/service
//	bench run    -task products -condition both -runs 3 -model sonnet
//	bench report -out results
//
// Run it from the benchmark directory. See README.md for the method.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var tasks = []string{"products", "products-sqlite", "blog", "orders"}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "score":
		err = cmdScore(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: bench score|run|report [flags] (see bench <command> -h)")
	os.Exit(2)
}

// cmdScore scores an existing service, such as a reference solution. Without
// -database-url, it creates fresh storage itself (PostgreSQL in Docker for the
// orders task).
func cmdScore(args []string) error {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	task := fs.String("task", "", "task: "+strings.Join(tasks, ", "))
	dir := fs.String("dir", "", "directory of the service to score")
	databaseURL := fs.String("database-url", "", "DATABASE_URL for the service (default: fresh storage)")
	testDatabaseURL := fs.String("test-database-url", "", "DATABASE_URL for the service's own tests")
	asJSON := fs.Bool("json", false, "print the score as JSON")
	_ = fs.Parse(args)
	if !slices.Contains(tasks, *task) || *dir == "" {
		return errors.New("-task and -dir are required")
	}

	st := storage{url: *databaseURL, testURL: *testDatabaseURL}
	if st.url == "" {
		var cleanup func()
		var err error
		st, cleanup, err = freshStorage(*task)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	if *task == "products-sqlite" {
		st.file = st.url
	}

	s := score(*task, *dir, st)
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(s)
	}
	for _, c := range s.Checks {
		mark := "✓"
		if !c.Passed {
			mark = "✗"
		}
		fmt.Printf("%s %s", mark, c.Name)
		if c.Detail != "" {
			fmt.Printf(": %s", c.Detail)
		}
		fmt.Println()
	}
	for _, note := range s.Notes {
		fmt.Println("note:", note)
	}
	fmt.Printf("\n%d/%d checks passed · built %t · vet %t · own tests %t · started %t\n",
		s.Passed, s.Total, s.Built, s.Vetted, s.OwnTestsPassed, s.Started)
	if s.Passed != s.Total {
		os.Exit(1)
	}
	return nil
}

// freshStorage creates empty storage for scoring on the host.
func freshStorage(task string) (storage, func(), error) {
	switch task {
	case "products-sqlite":
		dir, err := os.MkdirTemp("", "bench-sqlite-")
		if err != nil {
			return storage{}, func() {}, err
		}
		return storage{url: filepath.Join(dir, "data", "products.db"), testURL: filepath.Join(dir, "test", "products.db")},
			func() { os.RemoveAll(dir) }, nil
	case "orders":
		hostPort, err := ensurePostgres()
		if err != nil {
			return storage{}, func() {}, err
		}
		db, drop, err := createDatabase()
		if err != nil {
			return storage{}, func() {}, err
		}
		testDB, dropTest, err := createDatabase()
		if err != nil {
			drop()
			return storage{}, func() {}, err
		}
		return storage{url: hostDatabaseURL(hostPort, db), testURL: hostDatabaseURL(hostPort, testDB)},
			func() { drop(); dropTest() }, nil
	}
	return storage{}, func() {}, nil
}

// cmdRun runs agents on tasks in Docker and writes each result to the results
// directory.
func cmdRun(args []string) error {
	home, _ := os.UserHomeDir()
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	task := fs.String("task", "all", "task, or all: "+strings.Join(tasks, ", "))
	condition := fs.String("condition", "both", "scratch, goforge, or both")
	runs := fs.Int("runs", 1, "runs per task and condition")
	first := fs.Int("first-run", 1, "number of the first run, to add runs to earlier results")
	model := fs.String("model", "sonnet", "model, as passed to claude --model")
	budget := fs.Float64("budget", 5, "maximum spend per run, in US dollars")
	timeout := fs.Duration("timeout", 45*time.Minute, "maximum time per run")
	claudeVersion := fs.String("claude-version", "2.1.308", "Claude Code version in the image")
	goforgeVersion := fs.String("goforge-version", "v0.2.0", "GoForge version in the image")
	envFile := fs.String("env-file", filepath.Join(home, ".config", "goforge-bench", "env"),
		"Docker env file that sets CLAUDE_CODE_OAUTH_TOKEN (from `claude setup-token`) or ANTHROPIC_API_KEY; only Docker reads it")
	out := fs.String("out", "results", "results directory")
	work := fs.String("work", filepath.Join(os.TempDir(), "goforge-bench"), "directory for run directories")
	_ = fs.Parse(args)

	selected := tasks
	if *task != "all" {
		if !slices.Contains(tasks, *task) {
			return fmt.Errorf("unknown task %q", *task)
		}
		selected = []string{*task}
	}
	conditions := []string{scratch, goforge}
	if *condition != "both" {
		if *condition != scratch && *condition != goforge {
			return fmt.Errorf("unknown condition %q", *condition)
		}
		conditions = []string{*condition}
	}

	// The key stays in the file; Docker reads it into the agent's container
	if _, err := os.Stat(*envFile); err != nil {
		return fmt.Errorf("no credentials file at %s: create it with one line, CLAUDE_CODE_OAUTH_TOKEN=<token from `claude setup-token`> (or ANTHROPIC_API_KEY=<key>), and chmod 600 it", *envFile)
	}
	if _, err := ensurePostgres(); err != nil {
		return err
	}
	image, imageID, err := ensureImage(*claudeVersion, *goforgeVersion)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*work, 0o755); err != nil {
		return err
	}
	benchLinux, err := buildLinuxBench(*work)
	if err != nil {
		return err
	}

	// Alternate conditions within each run, so changes over time (such as API
	// load) affect both alike
	for run := *first; run < *first+*runs; run++ {
		for _, t := range selected {
			for _, c := range conditions {
				fmt.Printf("▶ %s · %s · run %d\n", t, c, run)
				r, err := runAgent(runConfig{
					task: t, condition: c, run: run, model: *model, budgetUSD: *budget, timeout: *timeout,
					image: image, imageID: imageID, benchLinux: benchLinux, envFile: *envFile, workRoot: *work,
				})
				if err != nil {
					return fmt.Errorf("%s %s run %d: %w", t, c, run, err)
				}
				if err := writeResult(*out, r); err != nil {
					return err
				}
				fmt.Printf("  %s · $%.2f · %d turns · %.1f min · %d/%d checks · goforge used %t\n",
					r.Agent.Outcome, r.Agent.CostUSD, r.Agent.Turns, float64(r.Agent.DurationMS)/60000,
					r.Score.Passed, r.Score.Total, r.Agent.GoforgeCommands > 0)
			}
		}
	}
	return nil
}

func writeResult(out string, r Result) error {
	dir := filepath.Join(out, r.Task)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-run%d.json", r.Condition, r.Run)), append(data, '\n'), 0o644)
}
