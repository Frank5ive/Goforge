package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// cmdReport summarizes the results directory as Markdown, printing it and
// writing it to REPORT.md in the results directory.
func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	out := fs.String("out", "results", "results directory")
	_ = fs.Parse(args)

	results, err := readResults(*out)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no results in %s", *out)
	}

	var b strings.Builder
	writeReport(&b, results)
	fmt.Print(b.String())
	return os.WriteFile(filepath.Join(*out, "REPORT.md"), []byte(b.String()), 0o644)
}

func readResults(dir string) ([]Result, error) {
	var results []Result
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var r Result
		if err := json.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		results = append(results, r)
		return nil
	})
	return results, err
}

// summary aggregates the runs of one task and condition.
type summary struct {
	runs, fullPasses, ownTests, usedGoforge                 int
	accept, cost, tokens, turns, minutes, failed, toolCalls float64
}

func summarize(results []Result) summary {
	var s summary
	for _, r := range results {
		s.runs++
		if r.Score.Total > 0 {
			s.accept += float64(r.Score.Passed) / float64(r.Score.Total)
			if r.Score.Passed == r.Score.Total {
				s.fullPasses++
			}
		}
		if r.Score.OwnTestsPassed {
			s.ownTests++
		}
		if r.Agent.GoforgeGenerated > 0 {
			s.usedGoforge++
		}
		s.cost += r.Agent.CostUSD
		s.tokens += float64(r.Agent.InputTokens + r.Agent.OutputTokens + r.Agent.CacheRead + r.Agent.CacheWrite)
		s.turns += float64(r.Agent.Turns)
		s.minutes += float64(r.Agent.DurationMS) / 60000
		s.failed += float64(r.Agent.FailedGoCommands)
		s.toolCalls += float64(r.Agent.ToolCalls)
	}
	n := float64(max(s.runs, 1))
	s.accept, s.cost, s.tokens, s.turns, s.minutes, s.failed, s.toolCalls =
		s.accept/n, s.cost/n, s.tokens/n, s.turns/n, s.minutes/n, s.failed/n, s.toolCalls/n
	return s
}

func writeReport(w io.Writer, results []Result) {
	byTask := map[string]map[string][]Result{}
	models := map[string]bool{}
	var labels []string
	for _, r := range results {
		l := label(r.Condition, r.GoforgeVersion)
		if byTask[r.Task] == nil {
			byTask[r.Task] = map[string][]Result{}
		}
		byTask[r.Task][l] = append(byTask[r.Task][l], r)
		models[r.Model] = true
		if !slices.Contains(labels, l) {
			labels = append(labels, l)
		}
	}
	slices.SortFunc(labels, compareLabels)

	var modelNames []string
	for m := range models {
		modelNames = append(modelNames, m)
	}
	slices.Sort(modelNames)
	fmt.Fprintf(w, "# GoForge benchmark results\n\nModel: %s. Values are means per run unless noted.\n\n", strings.Join(modelNames, ", "))

	fmt.Fprintln(w, "| Task | Condition | Runs | Checks passed | Fully correct | Cost | Tokens | Turns | Minutes | Failed go commands | Own tests pass | Used GoForge |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|---|---|---|---|")
	var overall = map[string][]Result{}
	for _, task := range tasks {
		for _, l := range labels {
			rs := byTask[task][l]
			if len(rs) == 0 {
				continue
			}
			overall[l] = append(overall[l], rs...)
			writeRow(w, task, l, summarize(rs))
		}
	}
	for _, l := range labels {
		if rs := overall[l]; len(rs) > 0 {
			writeRow(w, "**all tasks**", l, summarize(rs))
		}
	}

	fmt.Fprint(w, "\n## Runs\n\n")
	fmt.Fprintln(w, "| Task | Condition | Run | Outcome | Checks | Cost | Tokens | Turns | Minutes | Failed go commands | Denied tools |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|---|---|---|")
	slices.SortFunc(results, func(a, b Result) int {
		if c := strings.Compare(a.Task, b.Task); c != 0 {
			return c
		}
		if c := compareLabels(label(a.Condition, a.GoforgeVersion), label(b.Condition, b.GoforgeVersion)); c != 0 {
			return c
		}
		return a.Run - b.Run
	})
	for _, r := range results {
		a := r.Agent
		fmt.Fprintf(w, "| %s | %s | %d | %s | %d/%d | $%.2f | %s | %d | %.1f | %d | %d |\n",
			r.Task, label(r.Condition, r.GoforgeVersion), r.Run, a.Outcome, r.Score.Passed, r.Score.Total, a.CostUSD,
			thousands(a.InputTokens+a.OutputTokens+a.CacheRead+a.CacheWrite), a.Turns,
			float64(a.DurationMS)/60000, a.FailedGoCommands, a.PermissionDenials)
	}

	fmt.Fprint(w, "\n## Failed checks\n\n")
	listed := false
	for _, r := range results {
		for _, c := range r.Score.Checks {
			if !c.Passed {
				listed = true
				fmt.Fprintf(w, "- %s · %s · run %d: **%s**: %s\n", r.Task, label(r.Condition, r.GoforgeVersion), r.Run, c.Name, oneLine(c.Detail))
			}
		}
		for _, note := range r.Score.Notes {
			listed = true
			fmt.Fprintf(w, "- %s · %s · run %d: %s\n", r.Task, label(r.Condition, r.GoforgeVersion), r.Run, oneLine(note))
		}
	}
	if !listed {
		fmt.Fprintln(w, "None.")
	}
}

func writeRow(w io.Writer, task, condition string, s summary) {
	fmt.Fprintf(w, "| %s | %s | %d | %.0f%% | %d/%d | $%.2f | %s | %.0f | %.1f | %.1f | %d/%d | %d/%d |\n",
		task, condition, s.runs, s.accept*100, s.fullPasses, s.runs, s.cost, thousands(int(s.tokens)),
		s.turns, s.minutes, s.failed, s.ownTests, s.runs, s.usedGoforge, s.runs)
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// compareLabels orders condition labels: the scratch conditions first, then
// each GoForge version (released "v" versions before commits), with goforge
// before goforge-required.
func compareLabels(a, b string) int {
	ca, va, _ := strings.Cut(a, "@")
	cb, vb, _ := strings.Cut(b, "@")
	if usesGoforge(ca) != usesGoforge(cb) {
		if !usesGoforge(ca) {
			return -1
		}
		return 1
	}
	if va != vb {
		if strings.HasPrefix(va, "v") != strings.HasPrefix(vb, "v") {
			if strings.HasPrefix(va, "v") {
				return -1
			}
			return 1
		}
		return strings.Compare(va, vb)
	}
	return slices.Index(knownConditions, ca) - slices.Index(knownConditions, cb)
}
