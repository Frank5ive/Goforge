package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveAllReadOnlyModuleCache(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	mod := filepath.Join(home, "go", "pkg", "mod", "example.com", "m@v1.0.0")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "m.go"), []byte("package m\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	// Go makes module cache directories read-only
	for _, dir := range []string{mod, filepath.Dir(mod)} {
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
	}

	if err := removeAll(home); err != nil {
		t.Fatalf("removeAll() error = %v", err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Error("home still exists")
	}
}

func TestParseTranscriptError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	limit := `{"type":"result","subtype":"success","is_error":true,"result":"You've hit your session limit","num_turns":1}` + "\n"
	if err := os.WriteFile(path, []byte(limit), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := parseTranscript(path)
	if err != nil || a.Error != "You've hit your session limit" {
		t.Errorf("parseTranscript() = %+v, %v; want the error", a, err)
	}
}

func TestParseTranscriptCountsWhatTheAgentDid(t *testing.T) {
	lines := []string{
		`not json, skipped`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"goforge generate --name shop"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"cd service && go test ./..."}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t3","name":"Write","input":{}},{"type":"text"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t4","name":"Bash","input":{"command":"goforge add resource product"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t2","is_error":true},{"type":"tool_result","tool_use_id":"t1","is_error":true}]}}`,
		`{"type":"result","subtype":"error_max_budget_usd","total_cost_usd":1.25,"duration_ms":60000,"num_turns":9,` +
			`"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":30,"cache_creation_input_tokens":40},"permission_denials":[{},{}]}`,
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := parseTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Agent{Outcome: "error_max_budget_usd", CostUSD: 1.25, DurationMS: 60000, Turns: 9,
		InputTokens: 10, OutputTokens: 20, CacheRead: 30, CacheWrite: 40, PermissionDenials: 2,
		ToolCalls: 4, GoCommands: 1, GoforgeCommands: 2, GoforgeGenerated: 2,
		// Only the failed go command counts: goforge generate isn't one
		FailedGoCommands: 1}
	if a != want {
		t.Errorf("parseTranscript() =\n%+v\nwant\n%+v", a, want)
	}

	if _, err := parseTranscript(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("parseTranscript() of a missing file = nil, want an error")
	}
}

func TestIsGoCommand(t *testing.T) {
	for command, want := range map[string]bool{
		"go build ./...":           true,
		"cd service && go test -v": true,
		"goforge check --json":     true,
		"goforge generate -n shop": false,
		"ls -la":                   false,
		"echo 'go is fun'":         false,
		"go vet ./... && go run .": true,
	} {
		if got := isGoCommand(command); got != want {
			t.Errorf("isGoCommand(%q) = %t, want %t", command, got, want)
		}
	}
}

func TestBuildPromptDiffersOnlyInTheConditionText(t *testing.T) {
	const spec = "# Spec\nThings."
	prompts := map[string]string{}
	for _, c := range knownConditions {
		prompts[c] = buildPrompt(c, spec, "postgres://dev")
		for _, want := range []string{spec, "postgres://dev", "Listen on the port given by the PORT"} {
			if !strings.Contains(prompts[c], want) {
				t.Errorf("%s prompt lacks %q", c, want)
			}
		}
	}
	if strings.Contains(prompts[scratch], "goforge") || strings.Contains(prompts[scratch], "production-ready") {
		t.Error("the scratch prompt mentions GoForge or production readiness")
	}
	if !strings.Contains(prompts[scratchProduction], "graceful") || strings.Contains(prompts[scratchProduction], "goforge") {
		t.Error("the scratch-production prompt should ask for production features, without GoForge")
	}
	if !strings.Contains(prompts[goforge], "is installed") || strings.Contains(prompts[goforge], "Build the service with GoForge") {
		t.Error("the goforge prompt should offer GoForge, not require it")
	}
	if !strings.Contains(prompts[goforgeRequired], "Build the service with GoForge") {
		t.Error("the goforge-required prompt should require GoForge")
	}
	if strings.Contains(buildPrompt(scratch, spec, ""), "PostgreSQL database is available") {
		t.Error("a task without a database should not mention one")
	}
}

func TestUsesGoforge(t *testing.T) {
	for condition, want := range map[string]bool{scratch: false, scratchProduction: false, goforge: true, goforgeRequired: true} {
		if got := usesGoforge(condition); got != want {
			t.Errorf("usesGoforge(%q) = %t, want %t", condition, got, want)
		}
	}
}
