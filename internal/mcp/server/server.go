package server

import (
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// NewMCPServer wraps g's registered tools (see DefineTools) as an MCP
// server.
//
// Only stdio transport is actually available: verified by reading
// plugins/mcp/server.go (github.com/firebase/genkit/go v1.10.0)
// directly — GenkitMCPServer.Serve(transport) ignores its transport
// argument and always calls the stdio-only server.ServeStdio, and the
// pinned github.com/mark3labs/mcp-go v0.29.0's StreamableHTTPServer
// type is an explicit upstream "TODO: stub implementation" with every
// method a no-op. Streamable HTTP is a documented gap in this phase,
// not an oversight — PLAN.md's "stdio + Streamable HTTP" assumption
// doesn't hold against the live SDK.
func NewMCPServer(g *genkit.Genkit, name string) *mcp.GenkitMCPServer {
	return mcp.NewMCPServer(g, mcp.MCPServerOptions{Name: name})
}
