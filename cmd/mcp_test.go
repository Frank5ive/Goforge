package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connect returns an MCP client session connected in memory to a new server,
// the way an agent's client would be over stdio.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := newMCPServer("test")
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// call runs a tool and returns its decoded JSON result and whether it was an error.
func call(t *testing.T, session *mcp.ClientSession, tool string, args map[string]any) (result, bool) {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", tool, err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool(%s) returned %d content items, want 1", tool, len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s) content is %T, want text", tool, res.Content[0])
	}
	var r result
	if err := json.Unmarshal([]byte(text.Text), &r); err != nil {
		t.Fatalf("CallTool(%s) text is not the JSON result: %v\n%s", tool, err, text.Text)
	}
	return r, res.IsError
}

func TestMCPListsATool_ForEachCommand(t *testing.T) {
	session := connect(t)

	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range tools.Tools {
		byName[tool.Name] = tool
		if tool.Description == "" {
			t.Errorf("%s has no description, so an agent can't tell when to use it", tool.Name)
		}
	}
	want := []string{"goforge_add_db", "goforge_add_resource", "goforge_check", "goforge_generate", "goforge_inspect"}
	var got []string
	for name := range byName {
		got = append(got, name)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}

	// Required inputs are in each schema, so a client can validate before calling
	required := func(name string) []string {
		schema, _ := json.Marshal(byName[name].InputSchema)
		var s struct {
			Required []string `json:"required"`
		}
		_ = json.Unmarshal(schema, &s)
		slices.Sort(s.Required)
		return s.Required
	}
	for name, want := range map[string][]string{
		"goforge_generate":     {"name"},
		"goforge_add_resource": {"dir", "fields", "name"},
		"goforge_add_db":       {"dir", "engine"},
		"goforge_inspect":      {"dir"},
		"goforge_check":        {"dir"},
	} {
		if got := required(name); !slices.Equal(got, want) {
			t.Errorf("%s requires %v, want %v", name, got, want)
		}
	}

	// Reading is marked so a client can run inspect and check without asking
	if !byName["goforge_inspect"].Annotations.ReadOnlyHint || byName["goforge_generate"].Annotations.ReadOnlyHint {
		t.Error("inspect should be marked read-only and generate should not")
	}
}

func TestMCPBuildsAProjectInOneCall(t *testing.T) {
	session := connect(t)
	dir := t.TempDir()

	r, isError := call(t, session, "goforge_generate", map[string]any{
		"name": "shop", "module": "github.com/acme/shop", "output": dir, "database": "sqlite",
		"resources": []string{
			"category/categories:name:string",
			"product:name:string(1..100),price:float(0..),category_id:ref(category)",
		},
	})
	if isError || !r.OK || r.Command != "generate" {
		t.Fatalf("generate = %+v (error %t)", r, isError)
	}
	for _, path := range []string{"categories/service.go", "products/service.go", "openapi/openapi.json", "db"} {
		if _, err := os.Stat(filepath.Join(dir, "shop", path)); err != nil {
			t.Errorf("%s was not created: %v", path, err)
		}
	}

	// The agent can then look at what it built, and extend it
	project := filepath.Join(dir, "shop")
	var info inspectResult
	text := callText(t, session, "goforge_inspect", map[string]any{"dir": project})
	if err := json.Unmarshal([]byte(text), &info); err != nil || len(info.Resources) != 2 || info.Database != "sqlite" {
		t.Fatalf("inspect = %s (%v), want two resources on sqlite", text, err)
	}

	r, isError = call(t, session, "goforge_add_resource", map[string]any{
		"dir": project, "name": "review", "fields": "product_id:ref(product),rating:int(1..5)",
	})
	if isError || !r.OK || !slices.Contains(r.Created, "reviews/service.go") {
		t.Errorf("add_resource = %+v (error %t)", r, isError)
	}
}

// callText is call without decoding, for results with their own shape.
func callText(t *testing.T, session *mcp.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil || len(res.Content) != 1 {
		t.Fatalf("CallTool(%s) = %+v, %v", tool, res, err)
	}
	return res.Content[0].(*mcp.TextContent).Text
}

func TestMCPFailuresAreToolErrorsWithTheCodeAndHint(t *testing.T) {
	session := connect(t)

	r, isError := call(t, session, "goforge_inspect", map[string]any{"dir": t.TempDir()})
	if !isError || r.OK || r.Error == nil || r.Error.Code != "not_a_goforge_project" || r.Error.Hint == "" {
		t.Errorf("inspect of an empty directory = %+v (error %t), want a tool error with a code and a hint", r, isError)
	}

	dir := t.TempDir()
	r, isError = call(t, session, "goforge_generate", map[string]any{
		"name": "shop", "output": dir, "resources": []string{"product:name:string", "product:title:string"},
	})
	if !isError || r.Error == nil || r.Error.Code != "invalid_input" {
		t.Errorf("generate with a duplicate resource = %+v (error %t), want invalid_input", r, isError)
	}
	if _, err := os.Stat(filepath.Join(dir, "shop")); !os.IsNotExist(err) {
		t.Error("a failed generate left a project behind")
	}
}

func TestMCPDryRunWritesNothing(t *testing.T) {
	session := connect(t)
	dir := t.TempDir()

	r, isError := call(t, session, "goforge_generate", map[string]any{"name": "shop", "output": dir, "dry_run": true})
	if isError || !r.OK || !r.DryRun || len(r.Created) == 0 {
		t.Fatalf("dry run = %+v (error %t)", r, isError)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("a dry run wrote %d entries", len(entries))
	}
}

func TestMCPGenerateSwitchesOffFeatures(t *testing.T) {
	session := connect(t)
	dir := t.TempDir()
	off := false

	r, isError := call(t, session, "goforge_generate", map[string]any{"name": "shop", "output": dir, "docker": off, "ci": off})
	if isError || !r.OK {
		t.Fatalf("generate = %+v (error %t)", r, isError)
	}
	for _, path := range []string{"Dockerfile", ".github"} {
		if _, err := os.Stat(filepath.Join(dir, "shop", path)); !os.IsNotExist(err) {
			t.Errorf("%s was created although it was switched off", path)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "shop", "README.md")); err != nil {
		t.Errorf("README.md is on by default but was not created: %v", err)
	}
}

func TestMCPServerShowsInstructions(t *testing.T) {
	session := connect(t)
	init := session.InitializeResult()
	if init == nil || !strings.Contains(init.Instructions, "goforge_generate") || init.ServerInfo.Name != "goforge" {
		t.Errorf("initialize result = %+v, want the server name and instructions naming goforge_generate", init)
	}
}

func TestRunGoforgeReportsOutputThatIsNotJSON(t *testing.T) {
	// An unknown command prints usage text, not a result; the tool must still return JSON
	res := runGoforge("no-such-command")
	text := res.Content[0].(*mcp.TextContent).Text
	if !res.IsError || !json.Valid([]byte(text)) {
		t.Fatalf("runGoforge(no-such-command) = error %t, %q; want a JSON error result", res.IsError, text)
	}
	var r result
	if err := json.Unmarshal([]byte(text), &r); err != nil || r.Error == nil || r.Error.Hint == "" {
		t.Errorf("result = %+v (%v), want an error with a hint, as the docs promise for every failure", r, err)
	}
}

func TestEveryErrorCodeHasAHint(t *testing.T) {
	for _, code := range []string{codeUsage, codeCancelled, "invalid_input", "not_a_goforge_project", "already_exists",
		"marker_missing", "outdated_project", "dependency_failed", "write_failed", "internal_error"} {
		if hints[code] == "" {
			t.Errorf("the error code %s has no hint", code)
		}
	}
}

func TestServeMCPUntilTheClientLeaves(t *testing.T) {
	SetTemplatesFS(os.DirFS(filepath.Join("..", "templates")))
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- serveMCP(t.Context(), "test", serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil || len(tools.Tools) != 5 {
		t.Fatalf("ListTools() = %d tools, %v; want 5", len(tools.Tools), err)
	}
	_ = session.Close()

	if err := <-done; err != nil {
		t.Errorf("serveMCP() after the client left = %v, want nil", err)
	}
}

func TestMCPAddDBAndCheckAndOptions(t *testing.T) {
	session := connect(t)
	dir := t.TempDir()

	// Port and git are passed through to the command
	r, isError := call(t, session, "goforge_generate", map[string]any{"name": "shop", "output": dir, "port": 3000, "git": true})
	if isError || !r.OK {
		t.Fatalf("generate = %+v (error %t)", r, isError)
	}
	project := filepath.Join(dir, "shop")
	if config, err := os.ReadFile(filepath.Join(project, "config", "config.go")); err != nil || !strings.Contains(string(config), "3000") {
		t.Errorf("the port was not passed through: %v", err)
	}
	if _, err := exec.LookPath("git"); err == nil {
		if _, err := os.Stat(filepath.Join(project, ".git")); err != nil {
			t.Errorf("git was not initialized: %v", err)
		}
	}

	// A plural and a dry run are passed through to add resource
	r, isError = call(t, session, "goforge_add_resource", map[string]any{
		"dir": project, "name": "person", "fields": "name:string", "plural": "people", "dry_run": true,
	})
	if isError || !r.DryRun || !slices.Contains(r.Created, "people/service.go") {
		t.Errorf("add_resource dry run = %+v (error %t), want people/service.go planned", r, isError)
	}
	if _, err := os.Stat(filepath.Join(project, "people")); !os.IsNotExist(err) {
		t.Error("a dry run created the resource")
	}

	// A dry run of add_db shows what a database would change, and changes nothing
	before := snapshotFiles(t, project)
	r, isError = call(t, session, "goforge_add_db", map[string]any{"dir": project, "engine": "sqlite", "dry_run": true})
	if isError || !r.OK || !r.DryRun || len(r.Created)+len(r.Updated) == 0 {
		t.Errorf("add_db dry run = %+v (error %t)", r, isError)
	}
	if after := snapshotFiles(t, project); !slices.Equal(before, after) {
		t.Errorf("a dry run changed the project:\nbefore %v\nafter  %v", before, after)
	}
	if r, isError = call(t, session, "goforge_add_db", map[string]any{"dir": project, "engine": "oracle"}); !isError || r.Error == nil || r.Error.Code != "invalid_input" {
		t.Errorf("add_db with an unknown engine = %+v (error %t), want invalid_input", r, isError)
	}

	// A new project passes its own checks
	text := callText(t, session, "goforge_check", map[string]any{"dir": project})
	var report struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(text), &report); err != nil || !report.OK {
		t.Errorf("check = %s (%v), want ok", text, err)
	}
}

// snapshotFiles lists every file under dir with its size, sorted.
func snapshotFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return err
		}
		info, err := d.Info()
		files = append(files, path+" "+strconv.FormatInt(info.Size(), 10))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	return files
}
