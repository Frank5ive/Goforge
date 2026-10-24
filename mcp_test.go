package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPOverStdio runs the real binary as `goforge mcp` and talks to it the
// way an agent's MCP client does, over its standard input and output. Anything
// else the program wrote to standard output would corrupt the protocol.
func TestMCPOverStdio(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the stdio MCP test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found in PATH")
	}

	binary := filepath.Join(t.TempDir(), "goforge")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	runGo(t, ".", "build", "-o", binary, ".")

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: exec.Command(binary, "mcp")}, nil)
	if err != nil {
		t.Fatalf("connecting to goforge mcp: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	tools, err := session.ListTools(t.Context(), nil)
	if err != nil || len(tools.Tools) != 5 {
		t.Fatalf("ListTools() = %d tools, %v; want 5", len(tools.Tools), err)
	}

	dir := t.TempDir()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "goforge_generate", Arguments: map[string]any{
		"name": "shop", "module": "github.com/acme/shop", "output": dir,
		"resources": []string{"product:name:string(1..100),price:float(0..)"},
	}})
	if err != nil || res.IsError || len(res.Content) != 1 {
		t.Fatalf("goforge_generate = %+v, %v", res, err)
	}
	var generated struct {
		OK      bool     `json:"ok"`
		Created []string `json:"created"`
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &generated); err != nil || !generated.OK {
		t.Fatalf("goforge_generate result = %s (%v)", text, err)
	}

	// What it generated builds and passes its own tests, checked through the same server
	project := filepath.Join(dir, "shop")
	res, err = session.CallTool(t.Context(), &mcp.CallToolParams{Name: "goforge_check", Arguments: map[string]any{"dir": project}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"ok": true`) {
		t.Fatalf("goforge_check = %+v, %v", res, err)
	}
}
