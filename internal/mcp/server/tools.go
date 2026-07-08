// Package server exposes the Second Brain as MCP tools via Genkit's
// plugins/mcp server, so external agents (Claude Desktop, etc.) can
// read/write it over the Model Context Protocol.
package server

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/embedding"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// toolErr logs the real error server-side and returns a fixed, opaque
// message safe to forward to an external MCP caller. Genkit's MCP
// server plugin (plugins/mcp/server.go) calls err.Error() verbatim
// into the tool result sent back over the wire — driver/SQL error
// text must never reach that path.
func toolErr(tool string, err error) error {
	slog.Error("mcp tool failed", "tool", tool, "err", err)
	return fmt.Errorf("%s: internal error", tool)
}

// Deps is what this package needs to back the Second Brain MCP tools —
// interfaces only, never a concrete postgres.Store or
// embedding.GenkitEmbedder, so this package never imports pgx or a
// specific model provider, and tests can substitute fakes.
type Deps struct {
	Store    secondbrain.Store
	Embedder embedding.Embedder
}

// Tools holds every registered tool, so callers (tests, in particular)
// can invoke one directly without going through a model's tool-call
// loop.
type Tools struct {
	GetProfile        *ai.ToolDef[getProfileInput, getProfileOutput]
	GetProjectContext *ai.ToolDef[getProjectContextInput, getProjectContextOutput]
	LogSession        *ai.ToolDef[logSessionInput, logSessionOutput]
	SearchMemory      *ai.ToolDef[searchMemoryInput, searchMemoryOutput]
}

type getProfileInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"description=Max rules to return (defaults to 100, capped at 200)"`
	Offset int `json:"offset,omitempty" jsonschema:"description=Number of rules to skip, for paging past the first page"`
}

type profileRuleView struct {
	SourceFile string `json:"sourceFile"`
	Heading    string `json:"heading"`
	Content    string `json:"content"`
	Overridden bool   `json:"overridden"`
}

type getProfileOutput struct {
	Rules []profileRuleView `json:"rules"`
}

type getProjectContextInput struct {
	ProjectPath string `json:"projectPath" jsonschema:"description=Absolute path of the project directory"`
}

type getProjectContextOutput struct {
	Summary string `json:"summary"`
	Found   bool   `json:"found" jsonschema:"description=False when no context is stored yet for this project"`
}

type logSessionInput struct {
	SessionID string `json:"sessionId"`
	Role      string `json:"role" jsonschema:"description=Either 'user' or 'agent'"`
	Content   string `json:"content"`
}

type logSessionOutput struct {
	ID int64 `json:"id"`
}

type searchMemoryInput struct {
	Query string `json:"query"`
	K     int    `json:"k,omitempty" jsonschema:"description=Number of results to return (defaults to 5)"`
}

type memoryEntryView struct {
	ID        int64  `json:"id"`
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

type searchMemoryOutput struct {
	Entries []memoryEntryView `json:"entries"`
}

// DefineTools registers the four Second Brain MCP tools on g, backed
// by deps, and returns them for direct invocation (tests) alongside
// normal model tool-calling. Genkit's MCP server plugin auto-discovers
// any tool registered on the same *genkit.Genkit instance — these
// don't need separate MCP-specific registration.
func DefineTools(g *genkit.Genkit, deps Deps) Tools {
	return Tools{
		GetProfile: genkit.DefineTool(g, "secondbrain.get_profile",
			"Returns every imported profile rule from the Second Brain.",
			func(ctx *ai.ToolContext, in getProfileInput) (getProfileOutput, error) {
				rules, err := deps.Store.ListProfileRules(ctx, in.Limit, in.Offset)
				if err != nil {
					return getProfileOutput{}, toolErr("get_profile", err)
				}
				out := getProfileOutput{Rules: make([]profileRuleView, len(rules))}
				for i, r := range rules {
					out.Rules[i] = profileRuleView{
						SourceFile: r.SourceFile, Heading: r.Heading,
						Content: r.Content, Overridden: r.Overridden,
					}
				}
				return out, nil
			},
		),

		GetProjectContext: genkit.DefineTool(g, "secondbrain.get_project_context",
			"Returns the stored summary for a project directory, if one exists.",
			func(ctx *ai.ToolContext, in getProjectContextInput) (getProjectContextOutput, error) {
				pc, err := deps.Store.GetProjectContext(ctx, in.ProjectPath)
				if errors.Is(err, secondbrain.ErrNotFound) {
					return getProjectContextOutput{Found: false}, nil
				}
				if err != nil {
					return getProjectContextOutput{}, toolErr("get_project_context", err)
				}
				return getProjectContextOutput{Summary: pc.Summary, Found: true}, nil
			},
		),

		LogSession: genkit.DefineTool(g, "secondbrain.log_session",
			"Appends one turn of a session transcript to the Second Brain.",
			func(ctx *ai.ToolContext, in logSessionInput) (logSessionOutput, error) {
				entry, err := deps.Store.CreateMemoryEntry(ctx, secondbrain.MemoryEntry{
					SessionID: in.SessionID, Role: in.Role, Content: in.Content,
				})
				if err != nil {
					return logSessionOutput{}, toolErr("log_session", err)
				}
				return logSessionOutput{ID: entry.ID}, nil
			},
		),

		SearchMemory: genkit.DefineTool(g, "secondbrain.search_memory",
			"Finds the k most semantically similar session memory entries to query.",
			func(ctx *ai.ToolContext, in searchMemoryInput) (searchMemoryOutput, error) {
				k := in.K
				if k <= 0 {
					k = 5
				}
				vector, err := deps.Embedder.Embed(ctx, in.Query)
				if err != nil {
					return searchMemoryOutput{}, toolErr("search_memory", err)
				}
				entries, err := deps.Store.SearchMemoryEntriesByVector(ctx, vector, k)
				if err != nil {
					return searchMemoryOutput{}, toolErr("search_memory", err)
				}
				out := searchMemoryOutput{Entries: make([]memoryEntryView, len(entries))}
				for i, e := range entries {
					out.Entries[i] = memoryEntryView{ID: e.ID, SessionID: e.SessionID, Role: e.Role, Content: e.Content}
				}
				return out, nil
			},
		),
	}
}
