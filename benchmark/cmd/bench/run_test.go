package main

import (
	"os"
	"path/filepath"
	"slices"
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

func TestParseTranscriptRecordsWhatClaudeStartedWith(t *testing.T) {
	tests := []struct {
		name  string
		init  string
		skill bool
		mcp   bool
	}{
		{"nothing extra", `{"type":"system","subtype":"init","mcp_servers":[],"skills":["design","dataviz"]}`, false, false},
		{"the goforge skill", `{"type":"system","subtype":"init","mcp_servers":[],"skills":["design","goforge"]}`, true, false},
		{"the goforge server connected", `{"type":"system","subtype":"init","mcp_servers":[{"name":"goforge","status":"connected"}],"skills":[]}`, false, true},
		{"the goforge server failed", `{"type":"system","subtype":"init","mcp_servers":[{"name":"goforge","status":"failed"}],"skills":[]}`, false, false},
		{"another server", `{"type":"system","subtype":"init","mcp_servers":[{"name":"other","status":"connected"}],"skills":[]}`, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "transcript.jsonl")
			if err := os.WriteFile(path, []byte(tt.init+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			a, err := parseTranscript(path)
			if err != nil || a.SkillLoaded != tt.skill || a.MCPConnected != tt.mcp {
				t.Errorf("parseTranscript() = skill %t, mcp %t, %v; want %t, %t", a.SkillLoaded, a.MCPConnected, err, tt.skill, tt.mcp)
			}
		})
	}
}

func TestParseTranscriptCountsMCPToolCallsAsUsingGoforge(t *testing.T) {
	use := func(id, tool string) string {
		return `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"` + id + `","name":"` + tool + `","input":{}}]}}`
	}
	lines := []string{
		use("1", "mcp__goforge__goforge_generate"),
		use("2", "mcp__goforge__goforge_add_resource"),
		use("3", "mcp__goforge__goforge_add_db"),
		use("4", "mcp__goforge__goforge_inspect"),
		use("5", "mcp__goforge__goforge_check"),
		use("6", "mcp__other__goforge_generate"),
		use("7", "Write"),
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := parseTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	// Only generating and adding count as using GoForge, as for the commands;
	// inspect and check are looking, and another server's tool is not ours
	if a.GoforgeGenerated != 3 || a.GoforgeCommands != 5 || a.ToolCalls != 7 {
		t.Errorf("GoforgeGenerated = %d, GoforgeCommands = %d, ToolCalls = %d; want 3, 5, 7", a.GoforgeGenerated, a.GoforgeCommands, a.ToolCalls)
	}
}

func TestSkillAndMCPConditionsOfferGoforgeTheSameWay(t *testing.T) {
	const spec = "# Spec"
	offered := buildPrompt(goforge, spec, "")
	for _, c := range []string{goforgeSkill, goforgeMCP} {
		if got := buildPrompt(c, spec, ""); got != offered {
			t.Errorf("the %s prompt differs from the goforge prompt, so the condition would not isolate the skill or the server:\n%s", c, got)
		}
		if !usesGoforge(c) {
			t.Errorf("usesGoforge(%q) = false", c)
		}
	}
	for _, c := range []string{goforgeSkill, goforgeMCP} {
		if slices.Contains(conditions, c) {
			t.Errorf("%s runs with `all`, which would spend usage nobody asked for", c)
		}
		if !slices.Contains(knownConditions, c) {
			t.Errorf("%s is not a known condition", c)
		}
	}
}
