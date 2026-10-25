package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/benchmark/accept"
)

func result(task, condition, version string, run int, passed, total int) Result {
	return Result{
		Task: task, Condition: condition, GoforgeVersion: version, Run: run, Model: "sonnet",
		Agent: Agent{Outcome: "success", CostUSD: 0.10, InputTokens: 1000, OutputTokens: 500, CacheRead: 2000, CacheWrite: 500,
			Turns: 4, DurationMS: 90000, FailedGoCommands: 1, ToolCalls: 3},
		Score: Score{Passed: passed, Total: total, OwnTestsPassed: passed == total},
	}
}

func TestSummarizeAveragesRuns(t *testing.T) {
	used := result("products", goforgeRequired, "v1", 2, 10, 10)
	used.Agent.GoforgeGenerated = 1
	used.Agent.CostUSD = 0.30
	s := summarize([]Result{result("products", goforgeRequired, "v1", 1, 5, 10), used})

	if s.runs != 2 || s.fullPasses != 1 || s.ownTests != 1 || s.usedGoforge != 1 {
		t.Errorf("counts = %+v, want 2 runs, 1 full pass, 1 passing own tests, 1 using GoForge", s)
	}
	if s.accept != 0.75 || s.cost != 0.20 || s.tokens != 4000 || s.turns != 4 || s.minutes != 1.5 || s.failed != 1 {
		t.Errorf("means = %+v", s)
	}
	if empty := summarize(nil); empty.runs != 0 || empty.cost != 0 {
		t.Errorf("summarize(nil) = %+v, want zeros without dividing by zero", empty)
	}
}

func TestCompareLabelsOrdersConditions(t *testing.T) {
	labels := []string{"goforge-required@b632824", "goforge@v0.2.0", "scratch-production", "goforge-required@v0.2.0", "scratch", "goforge@b632824"}
	slices.SortFunc(labels, compareLabels)
	want := []string{"scratch", "scratch-production", "goforge@v0.2.0", "goforge-required@v0.2.0", "goforge@b632824", "goforge-required@b632824"}
	if !slices.Equal(labels, want) {
		t.Errorf("sorted labels = %v\nwant           %v", labels, want)
	}
}

func TestLabel(t *testing.T) {
	for _, tt := range []struct{ condition, version, want string }{
		{scratch, "v0.2.0", "scratch"},
		{scratchProduction, "v0.2.0", "scratch-production"},
		{goforge, "v0.2.0", "goforge@v0.2.0"},
		{goforgeRequired, "", "goforge-required"},
	} {
		if got := label(tt.condition, tt.version); got != tt.want {
			t.Errorf("label(%q, %q) = %q, want %q", tt.condition, tt.version, got, tt.want)
		}
	}
}

func TestThousandsAndOneLine(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567"} {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %q, want %q", in, got, want)
		}
	}
	if got := oneLine("a\n  b\t c"); got != "a b c" {
		t.Errorf("oneLine() = %q, want whitespace collapsed", got)
	}
	if got := oneLine(strings.Repeat("x", 400)); len([]rune(got)) != 301 || !strings.HasSuffix(got, "…") {
		t.Errorf("oneLine() of a long string has %d characters, want 300 and an ellipsis", len([]rune(got)))
	}
}

func TestWriteReport(t *testing.T) {
	failed := result("store", scratch, "", 1, 1, 2)
	failed.Score.Checks = []accept.Result{{Name: "deleting is a 409", Passed: false, Detail: "got\n500"}, {Name: "creating works", Passed: true}}
	failed.Score.Notes = []string{"go test failed: boom"}
	results := []Result{
		result("products", goforge, "v0.2.0", 1, 25, 25),
		result("products", scratch, "", 1, 25, 25),
		failed,
	}

	var b strings.Builder
	writeReport(&b, results)
	report := b.String()

	for _, want := range []string{
		"Model: sonnet.",
		"| products | scratch | 1 | 100% | 1/1 | $0.10 | 4,000 | 4 | 1.5 | 1.0 | 1/1 | 0/1 |",
		"| **all tasks** | scratch | 2 |",
		"| products | goforge@v0.2.0 | 1 |",
		"- store · scratch · run 1: **deleting is a 409**: got 500",
		"- store · scratch · run 1: go test failed: boom",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report doesn't contain %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "creating works") {
		t.Error("report lists a passing check as failed")
	}
	// The scratch row comes before the GoForge row
	if strings.Index(report, "| products | scratch |") > strings.Index(report, "| products | goforge@v0.2.0 |") {
		t.Error("scratch is not listed first")
	}

	b.Reset()
	writeReport(&b, results[:2])
	if !strings.Contains(b.String(), "None.") {
		t.Errorf("a report without failures should say so:\n%s", b.String())
	}
}

func TestReadResultsAndReport(t *testing.T) {
	dir := t.TempDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	for i, r := range []Result{result("products", scratch, "", 1, 25, 25), result("blog", goforge, "v0.2.0", 1, 23, 23)} {
		data, _ := json.Marshal(r)
		sub := filepath.Join(dir, r.Task)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "run"+string(rune('1'+i))+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Other files in the directory, such as an earlier report, are ignored
	if err := os.WriteFile(filepath.Join(dir, "REPORT.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := readResults(dir)
	if err != nil || len(results) != 2 {
		t.Fatalf("readResults() = %d results, %v; want 2", len(results), err)
	}

	if err := cmdReport([]string{"-out", dir}); err != nil {
		t.Fatalf("cmdReport() error = %v", err)
	}
	report, err := os.ReadFile(filepath.Join(dir, "REPORT.md"))
	if err != nil || !strings.Contains(string(report), "# GoForge benchmark results") {
		t.Errorf("REPORT.md = %q, %v", report, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readResults(dir); err == nil || !strings.Contains(err.Error(), "bad.json") {
		t.Errorf("readResults() with a broken file error = %v, want it to name the file", err)
	}

	if err := cmdReport([]string{"-out", t.TempDir()}); err == nil {
		t.Error("cmdReport() on an empty directory = nil, want an error")
	}
}

func TestCompareLabelsPlacesSkillAndMCPBetweenOfferedAndRequired(t *testing.T) {
	labels := []string{"goforge-required@v1", "goforge-mcp@v1", "goforge-skill@v1", "goforge@v1", "scratch"}
	slices.SortFunc(labels, compareLabels)
	want := []string{"scratch", "goforge@v1", "goforge-skill@v1", "goforge-mcp@v1", "goforge-required@v1"}
	if !slices.Equal(labels, want) {
		t.Errorf("sorted labels = %v, want %v", labels, want)
	}
}
