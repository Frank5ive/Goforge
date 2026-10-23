// Command bench runs the GoForge benchmark: AI agents build the same Go
// services from scratch and with GoForge, and the results are compared.
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
	"os/exec"
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

// cmdScore scores an existing service, such as a reference solution.
func cmdScore(args []string) error {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	task := fs.String("task", "", "task: "+strings.Join(tasks, ", "))
	dir := fs.String("dir", "", "directory of the service to score")
	_ = fs.Parse(args)
	if !slices.Contains(tasks, *task) || *dir == "" {
		return errors.New("-task and -dir are required")
	}

	s := score(*task, *dir)
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

// cmdRun runs agents on tasks and writes each result to the results directory.
func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	task := fs.String("task", "all", "task, or all: "+strings.Join(tasks, ", "))
	condition := fs.String("condition", "both", "scratch, goforge, or both")
	runs := fs.Int("runs", 1, "runs per task and condition")
	first := fs.Int("first-run", 1, "number of the first run, to add runs to earlier results")
	model := fs.String("model", "sonnet", "model, as passed to claude --model")
	budget := fs.Float64("budget", 5, "maximum spend per run, in US dollars")
	timeout := fs.Duration("timeout", 45*time.Minute, "maximum time per run")
	version := fs.String("goforge-version", "v0.2.0", "GoForge version to install for the goforge condition")
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

	goforgeBin, err := installGoforge(*version, filepath.Join(*work, "bin", *version))
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
					task: t, condition: c, run: run, model: *model, budgetUSD: *budget,
					timeout: *timeout, goforgeBin: goforgeBin, workRoot: *work,
				})
				if err != nil {
					return fmt.Errorf("%s %s run %d: %w", t, c, run, err)
				}
				if err := writeResult(*out, r); err != nil {
					return err
				}
				fmt.Printf("  %s · $%.2f · %d turns · %.1f min · %d/%d checks\n",
					r.Agent.Outcome, r.Agent.CostUSD, r.Agent.Turns, float64(r.Agent.DurationMS)/60000, r.Score.Passed, r.Score.Total)
			}
		}
	}
	return nil
}

// installGoforge installs the GoForge version into dir and returns dir.
func installGoforge(version, dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "goforge")); err == nil {
		return dir, nil
	}
	cmd := exec.Command("go", "install", "github.com/Frank5ive/goforge@"+version)
	cmd.Env = append(os.Environ(), "GOBIN="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("installing GoForge %s: %v: %s", version, err, out)
	}
	return dir, nil
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
