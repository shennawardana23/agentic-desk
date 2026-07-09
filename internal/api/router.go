// Package api is the Core↔GUI HTTP+WS surface: Gin routes passing
// through to internal/secondbrain, plus a WS channel (see hub.go) the
// GUI subscribes to for agent-thinking-log and HITL escalation
// events. Deps takes secondbrain.Store and embedding.Embedder as
// interfaces only, mirroring internal/mcp/server's own dependency
// shape — this package never imports pgx or a specific model
// provider.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/firebase/genkit/go/core"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/shennawardana23/agentic-desk/internal/chat"
	"github.com/shennawardana23/agentic-desk/internal/embedding"
	"github.com/shennawardana23/agentic-desk/internal/graph"
	"github.com/shennawardana23/agentic-desk/internal/library"
	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
	"github.com/shennawardana23/agentic-desk/internal/task"
	"github.com/shennawardana23/agentic-desk/internal/voicelive"
)

// voiceUpgrader is a dedicated WebSocket upgrader for the voice live endpoint.
// Larger buffers suit binary audio frames (1-4KB each at 32ms/chunk).
// TCP_NODELAY is set after upgrade to prevent Nagle buffering of small
// JSON control frames (transcript ~50-200B, interrupt ~40B).
var voiceUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// ChatFlow is what this package needs to run Sarza's conversational flow —
// an interface, not internal/orchestrator's concrete *core.Flow, so tests can
// substitute a fake without a real Gemini call (the same shape
// secondbrain.Store/embedding.Embedder already use). *core.Flow[ChatInput,
// ChatOutput, struct{}]'s real Run method already has exactly this
// signature (verified against github.com/firebase/genkit/go/core/flow.go),
// so genkit.DefineChatFlow's return value satisfies this with no adapter.
type ChatFlow interface {
	Run(ctx context.Context, in orchestrator.ChatInput) (orchestrator.ChatOutput, error)
}

// ChatStreamFlow is the streaming sibling of ChatFlow — the push-iterator
// shape *core.Flow[In, Out, Stream].Stream already has (verified against
// the pinned SDK's core/flow.go), so orchestrator.DefineChatFlows' Stream
// flow satisfies it with no adapter, and tests can substitute a fake.
type ChatStreamFlow interface {
	Stream(ctx context.Context, in orchestrator.ChatInput) func(func(*core.StreamingFlowValue[orchestrator.ChatOutput, orchestrator.ChatChunk], error) bool)
}

// Deps is what this package needs to serve the Core↔GUI API.
type Deps struct {
	Store    secondbrain.Store
	Embedder embedding.Embedder
	Hub      *Hub
	// Chat is nil-able: cmd/core only wires it once genkit.Init has a real
	// Genkit app to build the flow from. A nil Chat makes POST /chat return
	// 503 rather than panic — the same "surface a real error, don't fake
	// success" rule the rest of this API follows.
	Chat ChatFlow
	// ChatStream backs POST /chat/stream. Same nil-able contract as Chat.
	ChatStream ChatStreamFlow
	// Tasks backs the /tasks routes (Task Management). Nil-able, same
	// 503 contract as Chat.
	Tasks task.Store
	// ChatHistory backs the /chat/sessions routes (Chat History). Nil-able,
	// same 503 contract as Chat. Named ChatHistory (not Chat) because
	// ChatFlow already claims that name for the Sarza conversational flow.
	ChatHistory chat.Store
	// Graph backs GET /graph (Knowledge Graph). Nil-able → 503.
	Graph graph.Builder
	// Library backs the /skills and /prompts catalog routes. Nil-able → 503.
	Library *library.Library
	// VoiceLive backs GET /voice/live/ws (realtime Gemini Live voice
	// session) and GET /voice/live/config (model/voice catalog). Nil-able,
	// same 503 contract as Chat.
	VoiceLive *voicelive.Bridge
}

// apiErr logs the real error server-side and writes a fixed, opaque
// JSON error body — the same sanitization contract this session
// established for internal/mcp/server and internal/tools/github's
// tools, applied here since this API is the GUI's direct window into
// the same backing store.
func apiErr(c *gin.Context, status int, op string, err error) {
	slog.Error("api request failed", "op", op, "err", err)
	c.JSON(status, gin.H{"error": op + ": internal error"})
}

type profileRuleView struct {
	SourceFile string `json:"sourceFile"`
	Heading    string `json:"heading"`
	Content    string `json:"content"`
	Overridden bool   `json:"overridden"`
}

func newProfileRuleView(r secondbrain.ProfileRule) profileRuleView {
	return profileRuleView{SourceFile: r.SourceFile, Heading: r.Heading, Content: r.Content, Overridden: r.Overridden}
}

type memoryEntryView struct {
	ID        int64  `json:"id"`
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

func newMemoryEntryView(e secondbrain.MemoryEntry) memoryEntryView {
	return memoryEntryView{ID: e.ID, SessionID: e.SessionID, Role: e.Role, Content: e.Content}
}

// atoiOr parses s as an int, returning fallback on empty input or a
// parse error rather than failing the request — every caller of this
// helper treats the value as an optional pagination/search parameter
// with its own sensible default downstream.
func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

// corsOriginAllowed reports whether a browser Origin may read this API's
// responses: the Wails shell's own origin (wails://wails.localhost on
// macOS — the exact value TESTING.md's curl checks have used since
// iteration 4 — plus the http(s)://wails.localhost forms other platforms
// use) or a local dev server (wails dev serves the frontend from a
// localhost port). Everything else — i.e. any real website's origin —
// gets no CORS headers, so its cross-origin reads fail in the browser.
func corsOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	if strings.HasPrefix(origin, "wails://") {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "wails.localhost"
}

// NewRouter builds the Gin engine with every Core↔GUI route wired to
// deps.
func NewRouter(deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	// The Wails desktop shell serves its frontend from a wails:// origin,
	// not http://localhost:<APIAddr> — every fetch() from the GUI is
	// cross-origin, so without CORS headers WKWebView/WebView2 silently
	// drop the response before JS ever sees it (surfaces in the browser
	// console as "TypeError: Load failed" / "Failed to fetch", not as a
	// 4xx/5xx). The origin is echoed back only when it's the Wails shell
	// or a local dev server — NOT "*": with a wildcard, any website open
	// in the user's regular browser could fetch this loopback API
	// cross-origin and read profile rules/memories, or mutate tasks
	// (2026-07-07 security-review finding). Non-browser clients (curl,
	// the desktop launcher's health poll) send no Origin header and are
	// unaffected — CORS is a browser-side control either way.
	r.Use(func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); corsOriginAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/profile", func(c *gin.Context) {
		limit := atoiOr(c.Query("limit"), 0)
		offset := atoiOr(c.Query("offset"), 0)
		rules, err := deps.Store.ListProfileRules(c.Request.Context(), limit, offset)
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "list profile rules", err)
			return
		}
		views := make([]profileRuleView, len(rules))
		for i, rule := range rules {
			views[i] = newProfileRuleView(rule)
		}
		c.JSON(http.StatusOK, gin.H{"rules": views})
	})

	r.GET("/profile/rule", func(c *gin.Context) {
		sourceFile, heading := c.Query("sourceFile"), c.Query("heading")
		rule, err := deps.Store.GetProfileRule(c.Request.Context(), sourceFile, heading)
		if errors.Is(err, secondbrain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile rule not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "get profile rule", err)
			return
		}
		c.JSON(http.StatusOK, newProfileRuleView(rule))
	})

	r.GET("/project-context", func(c *gin.Context) {
		pc, err := deps.Store.GetProjectContext(c.Request.Context(), c.Query("projectPath"))
		if errors.Is(err, secondbrain.ErrNotFound) {
			c.JSON(http.StatusOK, gin.H{"found": false})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "get project context", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"found": true, "summary": pc.Summary})
	})

	r.PUT("/project-context", func(c *gin.Context) {
		var body struct {
			ProjectPath string `json:"projectPath" binding:"required"`
			Summary     string `json:"summary"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		pc, err := deps.Store.UpsertProjectContext(c.Request.Context(), secondbrain.ProjectContext{
			ProjectPath: body.ProjectPath, Summary: body.Summary,
		})
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "upsert project context", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"found": true, "summary": pc.Summary})
	})

	r.POST("/memory", func(c *gin.Context) {
		var body struct {
			SessionID string `json:"sessionId" binding:"required"`
			Role      string `json:"role" binding:"required"`
			Content   string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		entry, err := deps.Store.CreateMemoryEntry(c.Request.Context(), secondbrain.MemoryEntry{
			SessionID: body.SessionID, Role: body.Role, Content: body.Content,
		})
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "create memory entry", err)
			return
		}
		c.JSON(http.StatusCreated, newMemoryEntryView(entry))
	})

	r.GET("/memory/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		entry, err := deps.Store.GetMemoryEntry(c.Request.Context(), id)
		if errors.Is(err, secondbrain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "memory entry not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "get memory entry", err)
			return
		}
		c.JSON(http.StatusOK, newMemoryEntryView(entry))
	})

	r.GET("/memory/search", func(c *gin.Context) {
		query := c.Query("query")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
			return
		}
		k := atoiOr(c.Query("k"), 5)
		vector, err := deps.Embedder.Embed(c.Request.Context(), query)
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "embed query", err)
			return
		}
		entries, err := deps.Store.SearchMemoryEntriesByVector(c.Request.Context(), vector, k)
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "search memory", err)
			return
		}
		views := make([]memoryEntryView, len(entries))
		for i, e := range entries {
			views[i] = newMemoryEntryView(e)
		}
		c.JSON(http.StatusOK, gin.H{"entries": views})
	})

	r.POST("/chat", func(c *gin.Context) {
		if deps.Chat == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat: not configured"})
			return
		}
		var body orchestrator.ChatInput
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		out, err := deps.Chat.Run(c.Request.Context(), body)
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "chat", err)
			return
		}
		c.JSON(http.StatusOK, out)
	})

	// POST /chat/stream — SSE. Events, one JSON object per `data:` line:
	// {"type":"reasoning"|"text","content":...} per streamed chunk, then a
	// terminal {"type":"done","reply":...} or {"type":"error","error":...}
	// (sanitized, same contract as apiErr). Client abort tears down
	// c.Request.Context(), which cancels the in-flight generation — that is
	// the GUI's stop button end-to-end. (Genkit *tool* interrupts are a
	// different mechanism — human-in-the-loop tool pauses; the chat flow
	// defines no tools yet, so there is nothing to resolve here. See the
	// 2026-07-07 design doc.)
	r.POST("/chat/stream", func(c *gin.Context) {
		if deps.ChatStream == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat: not configured"})
			return
		}
		var body orchestrator.ChatInput
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		send := func(v any) {
			b, err := json.Marshal(v)
			if err != nil {
				slog.Error("chat stream: marshal event", "err", err)
				return
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", b)
			c.Writer.Flush()
		}

		for val, err := range deps.ChatStream.Stream(c.Request.Context(), body) {
			if err != nil {
				slog.Error("api request failed", "op", "chat stream", "err", err)
				send(gin.H{"type": "error", "error": "chat: internal error"})
				return
			}
			if val.Done {
				send(gin.H{"type": "done", "reply": val.Output.Reply})
				return
			}
			send(val.Stream)
		}
	})

	// Task Management routes — thin pass-throughs to task.Store, same
	// sanitized-error/503-when-unwired contract as everything above.
	requireTasks := func(c *gin.Context) bool {
		if deps.Tasks == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tasks: not configured"})
			return false
		}
		return true
	}

	r.GET("/tasks", func(c *gin.Context) {
		if !requireTasks(c) {
			return
		}
		tasks, err := deps.Tasks.List(c.Request.Context())
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "list tasks", err)
			return
		}
		if tasks == nil {
			tasks = []task.Task{}
		}
		c.JSON(http.StatusOK, gin.H{"tasks": tasks})
	})

	r.POST("/tasks", func(c *gin.Context) {
		if !requireTasks(c) {
			return
		}
		var body struct {
			Title       string `json:"title" binding:"required"`
			Notes       string `json:"notes"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		created, err := deps.Tasks.Create(c.Request.Context(), task.Task{
			Title: body.Title, Notes: body.Notes, Description: body.Description, Priority: body.Priority, Status: task.StatusTodo,
		})
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "create task", err)
			return
		}
		c.JSON(http.StatusCreated, created)
	})

	r.PUT("/tasks/:id", func(c *gin.Context) {
		if !requireTasks(c) {
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var body struct {
			Title       string `json:"title" binding:"required"`
			Notes       string `json:"notes"`
			Description string `json:"description"`
			Status      string `json:"status" binding:"required"`
			Priority    int    `json:"priority"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		updated, err := deps.Tasks.Update(c.Request.Context(), task.Task{
			ID: id, Title: body.Title, Notes: body.Notes, Description: body.Description, Status: body.Status, Priority: body.Priority,
		})
		if errors.Is(err, task.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "update task", err)
			return
		}
		c.JSON(http.StatusOK, updated)
	})

	r.DELETE("/tasks/:id", func(c *gin.Context) {
		if !requireTasks(c) {
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		err = deps.Tasks.Delete(c.Request.Context(), id)
		if errors.Is(err, task.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "delete task", err)
			return
		}
		c.Status(http.StatusNoContent)
	})

	// Chat History routes — thin pass-throughs to chat.Store, same
	// sanitized-error/503-when-unwired contract as everything above.
	requireChatHistory := func(c *gin.Context) bool {
		if deps.ChatHistory == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat history: not configured"})
			return false
		}
		return true
	}

	r.GET("/chat/sessions", func(c *gin.Context) {
		if !requireChatHistory(c) {
			return
		}
		sessions, err := deps.ChatHistory.ListSessions(c.Request.Context())
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "list chat sessions", err)
			return
		}
		if sessions == nil {
			sessions = []chat.ChatSession{}
		}
		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	})

	r.POST("/chat/sessions", func(c *gin.Context) {
		if !requireChatHistory(c) {
			return
		}
		created, err := deps.ChatHistory.CreateSession(c.Request.Context())
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "create chat session", err)
			return
		}
		c.JSON(http.StatusCreated, created)
	})

	r.PATCH("/chat/sessions/:id", func(c *gin.Context) {
		if !requireChatHistory(c) {
			return
		}
		var body struct {
			Title string `json:"title" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		updated, err := deps.ChatHistory.RenameSession(c.Request.Context(), c.Param("id"), body.Title)
		if errors.Is(err, chat.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chat session not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "rename chat session", err)
			return
		}
		c.JSON(http.StatusOK, updated)
	})

	r.DELETE("/chat/sessions/:id", func(c *gin.Context) {
		if !requireChatHistory(c) {
			return
		}
		err := deps.ChatHistory.DeleteSession(c.Request.Context(), c.Param("id"))
		if errors.Is(err, chat.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chat session not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "delete chat session", err)
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.GET("/chat/sessions/:id/messages", func(c *gin.Context) {
		if !requireChatHistory(c) {
			return
		}
		messages, err := deps.ChatHistory.ListMessages(c.Request.Context(), c.Param("id"))
		if errors.Is(err, chat.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chat session not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "list chat messages", err)
			return
		}
		if messages == nil {
			messages = []chat.ChatMessage{}
		}
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	})

	r.POST("/chat/sessions/:id/messages", func(c *gin.Context) {
		if !requireChatHistory(c) {
			return
		}
		var body struct {
			Role      string `json:"role" binding:"required"`
			Content   string `json:"content" binding:"required"`
			Reasoning string `json:"reasoning"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		created, err := deps.ChatHistory.AppendMessage(c.Request.Context(), c.Param("id"), body.Role, body.Content, body.Reasoning)
		if errors.Is(err, chat.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chat session not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "append chat message", err)
			return
		}
		c.JSON(http.StatusCreated, created)
	})

	// Knowledge Graph — a read-only projection of Second Brain data.
	r.GET("/graph", func(c *gin.Context) {
		if deps.Graph == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "graph: not configured"})
			return
		}
		data, err := deps.Graph.Build(c.Request.Context())
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "build graph", err)
			return
		}
		c.JSON(http.StatusOK, data)
	})

	// Skill Catalog + Prompt Catalog — browse-only filesystem reads.
	requireLibrary := func(c *gin.Context) bool {
		if deps.Library == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "library: not configured"})
			return false
		}
		return true
	}

	r.GET("/skills", func(c *gin.Context) {
		if !requireLibrary(c) {
			return
		}
		items, err := deps.Library.ListSkills()
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "list skills", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"skills": items})
	})

	r.GET("/skills/:name", func(c *gin.Context) {
		if !requireLibrary(c) {
			return
		}
		content, err := deps.Library.GetSkill(c.Param("name"))
		if errors.Is(err, library.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "get skill", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": c.Param("name"), "content": content})
	})

	r.GET("/prompts", func(c *gin.Context) {
		if !requireLibrary(c) {
			return
		}
		items, err := deps.Library.ListPrompts()
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "list prompts", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"prompts": items})
	})

	r.GET("/prompts/:name", func(c *gin.Context) {
		if !requireLibrary(c) {
			return
		}
		content, err := deps.Library.GetPrompt(c.Param("name"))
		if errors.Is(err, library.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found"})
			return
		}
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "get prompt", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": c.Param("name"), "content": content})
	})

	r.GET("/ws", func(c *gin.Context) {
		serveWS(c, deps.Hub)
	})

	// ── Agent Live voice API (matches reference /api/agent-live/* routes) ───────
	vl := deps.VoiceLive // shorthand

	// GET /api/agent-live/models — live-capable model catalog
	r.GET("/api/agent-live/models", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		c.JSON(http.StatusOK, gin.H{
			"models":      voicelive.AllModelsGrouped(),
			"live_models": voicelive.AllLiveModels(),
		})
	})

	// GET /api/agent-live/presets — system + user presets
	r.GET("/api/agent-live/presets", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		c.JSON(http.StatusOK, gin.H{"presets": vl.GetAllPresets()})
	})

	// POST /api/agent-live/presets — create user preset
	r.POST("/api/agent-live/presets", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		var req voicelive.CreatePresetRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return
		}
		p, err := vl.CreatePreset(c.Request.Context(), req)
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "create preset", err); return
		}
		c.JSON(http.StatusCreated, p)
	})

	// POST /api/agent-live/sessions — create session, return session_id
	r.POST("/api/agent-live/sessions", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		var req voicelive.CreateSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return
		}
		sess, err := vl.CreateSession(c.Request.Context(), req)
		if err != nil {
			apiErr(c, http.StatusInternalServerError, "create session", err); return
		}
		c.JSON(http.StatusCreated, sess)
	})

	// GET /api/agent-live/sessions — list all sessions
	r.GET("/api/agent-live/sessions", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		c.JSON(http.StatusOK, gin.H{"sessions": vl.ListSessions(c.Request.Context())})
	})

	// GET /api/agent-live/sessions/:id/stream — WebSocket upgrade for live session
	r.GET("/api/agent-live/sessions/:id/stream", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		conn, err := voiceUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Error("agent-live ws upgrade failed", "err", err); return
		}
		if tc, ok := conn.NetConn().(*net.TCPConn); ok { _ = tc.SetNoDelay(true) }
		defer conn.Close()
		sid := c.Param("id")
		cfg := voicelive.SessionConfig{
			VoiceName:   c.Query("voice_name"),
			SystemText:  c.Query("system_text"),
			Temperature: func() float32 {
				var t float32; fmt.Sscanf(c.Query("temperature"), "%f", &t); return t
			}(),
		}
		if err := vl.HandleStream(c.Request.Context(), sid, conn, cfg); err != nil {
			slog.Error("agent-live session error", "err", err, "session", sid)
		}
	})

	// POST /api/agent-live/sessions/:id/end — graceful end
	r.POST("/api/agent-live/sessions/:id/end", func(c *gin.Context) {
		if vl == nil { c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"}); return }
		if err := vl.EndSession(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()}); return
		}
		c.Status(http.StatusNoContent)
	})

	// ── Legacy voice endpoints (kept for backward compat with old frontend) ──────
	r.GET("/voice/live/config", func(c *gin.Context) {
		if deps.VoiceLive == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"defaultModel": voicelive.DefaultModel,
			"voices":       voicelive.Voices,
		})
	})

	r.GET("/voice/live/ws", func(c *gin.Context) {
		if deps.VoiceLive == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "voice live not configured"})
			return
		}
		conn, err := voiceUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Error("voice live ws upgrade failed", "err", err)
			return
		}
		if tc, ok := conn.NetConn().(*net.TCPConn); ok { _ = tc.SetNoDelay(true) }
		defer conn.Close()
		deps.VoiceLive.Serve(c.Request.Context(), conn)
	})

	return r
}
