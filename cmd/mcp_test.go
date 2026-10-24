package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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
		t.Errorf("runGoforge(no-such-command) = error %t, %q; want a JSON error result", res.IsError, text)
	}
}
