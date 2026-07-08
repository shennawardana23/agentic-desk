package server

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/firebase/genkit/go/genkit"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// TestMCPServer_RealClientRoundTrip proves PLAN.md Phase 7's own bar —
// "a real MCP client dials in, lists tools, round-trips one call" —
// against the real GenkitMCPServer (Genkit's documented way to expose
// tools as an MCP server, https://genkit.dev/docs/go/model-context-protocol/#exposing-as-mcp-server)
// over the real stdio wire protocol. The client side uses the official
// github.com/modelcontextprotocol/go-sdk, not mark3labs/mcp-go — this
// repo never imports mark3labs/mcp-go directly; it only arrives
// transitively as Genkit's own internal choice for plugins/mcp (see
// go.mod: marked `// indirect`).
//
// No DATABASE_URL, no GEMINI_API_KEY: Store/Embedder are fakes, so
// this is fully offline and always runs.
//
// GenkitMCPServer only exposes ServeStdio()/Serve(), both of which
// hardcode os.Stdin/os.Stdout internally (verified in server.go) —
// there's no constructor-level way to hand it a custom transport. So
// this test does the same swap any stdio-CLI test would: replace the
// process's os.Stdin/os.Stdout with pipes for its duration, restore
// them on exit, and never run in parallel with anything else that
// touches them (it doesn't).
func TestMCPServer_RealClientRoundTrip(t *testing.T) {
	store := newFakeStore()
	store.rules = []secondbrain.ProfileRule{{SourceFile: "CLAUDE.md", Heading: "A", Content: "a rule"}}

	g := genkit.Init(context.Background())
	DefineTools(g, Deps{Store: store, Embedder: fakeEmbedder{}})
	mcpServer := NewMCPServer(g, "agentic-desk-test")

	origStdin, origStdout := os.Stdin, os.Stdout
	stdinReadEnd, stdinWriteEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	stdoutReadEnd, stdoutWriteEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = stdinReadEnd    // server reads client requests here
	os.Stdout = stdoutWriteEnd // server writes responses here
	t.Cleanup(func() {
		os.Stdin, os.Stdout = origStdin, origStdout
	})

	serveErr := make(chan error, 1)
	go func() { serveErr <- mcpServer.ServeStdio() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "agentic-desk-test-client", Version: "0.0.0"}, nil)
	transport := &mcp.IOTransport{Reader: stdoutReadEnd, Writer: stdinWriteEnd}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make(map[string]bool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		names[tool.Name] = true
	}
	if len(toolsResult.Tools) != 4 {
		t.Errorf("expected exactly 4 tools listed, got %d: %v", len(toolsResult.Tools), names)
	}
	for _, want := range []string{
		"secondbrain.get_profile", "secondbrain.get_project_context",
		"secondbrain.log_session", "secondbrain.search_memory",
	} {
		if !names[want] {
			t.Errorf("expected tool %q to be listed, got %v", want, names)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "secondbrain.get_profile", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected a successful tool call, got an error result: %+v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty tool call content")
	}

	stdinWriteEnd.Close()
	select {
	case err := <-serveErr:
		if err != nil && err != io.EOF {
			t.Errorf("ServeStdio returned an unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("ServeStdio did not shut down after stdin closed")
	}
}
