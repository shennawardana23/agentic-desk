# Chat streaming (CoT + stop) and the five domain features — design

Date: 2026-07-07. Scope decisions confirmed with the user via AskUserQuestion this session
(all four "Recommended" options picked). This doc records what is being built, why each
shape was chosen, and the explicit non-goals.

## 1. Chat: streamed chain-of-thought, spinner, stop button

**Problem:** the chat UI shows a static italic "Sarza is thinking…" line, no reasoning,
no way to cancel a slow generation. User asked for: visible CoT while thinking, the
`brand/icon-desktop.svg` mark spinning instead of text/dots, and a stop button.

**Verified facts (pinned SDK `github.com/firebase/genkit/go@v1.10.0`, read from module cache):**
- `ai.PartReasoning` / `ai.NewReasoningPart` exist; `plugins/googlegenai/gemini.go:485`
  converts Gemini thought parts into reasoning parts, gated on the request config
  `thinkingConfig.includeThoughts` (`config_overrides.go`).
- `genkit.DefineStreamingFlow` + `ai.WithStreaming(cb)` exist; `core.Flow.Stream(ctx, in)`
  returns a push iterator suitable for an SSE handler.
- `plugins/compat_oai/generate.go` converts a `map[string]any` config via JSON into
  `openai.ChatCompletionNewParams` — unknown keys like `thinkingConfig` are silently
  dropped by `json.Unmarshal`, so enabling thoughts chain-wide does not break the
  non-Gemini fallback providers.
- `internal/api`'s `/chat` handler already passes `c.Request.Context()` into the flow, so
  an aborted HTTP request cancels generation server-side. The missing half is purely
  client-side (no `AbortController`) plus a streaming transport.

**Build:**
- `internal/orchestrator`: new `ChatChunk{Type: "reasoning"|"text", Content}` +
  `DefineChatStreamFlow(g)` — same message assembly/system prompt/Fallback+Retry stack as
  `DefineChatFlow`, plus `ai.WithStreaming` forwarding reasoning/text parts as typed
  chunks, plus `ai.WithConfig(map[string]any{"thinkingConfig": {"includeThoughts": true}})`.
  The non-streaming `DefineChatFlow` stays (voice + existing tests use it).
- `internal/api`: `POST /chat/stream` — SSE (`text/event-stream`); events:
  `data: {"type":"reasoning"|"text","content":...}` per chunk, then
  `data: {"type":"done","reply":...}`, or `data: {"type":"error","error":...}` (sanitized,
  same contract as `apiErr`). Flush per event. Client disconnect → request ctx cancel →
  generation stops.
- Frontend `stores/core.js`: `sendChatMessage` rewritten on `/chat/stream` with
  `fetch` + `ReadableStream` reader + `AbortController`; new state `chatThinking`
  (live reasoning text), streamed partial reply, `stopChat()`.
- `ChatView.vue`: thinking bubble = `icon-desktop.svg` rotating (CSS keyframes, disabled
  under `prefers-reduced-motion`) + live reasoning text in a collapsible section; agent
  reply streams into its bubble; finished messages keep their reasoning behind a
  collapsed "Thinking" toggle. Send button morphs into a stop (square) button while
  streaming; clicking aborts.

**Genkit interrupts (user asked to check, per genkit.dev/docs/go/interrupts):** those are
*tool* interrupts — human-in-the-loop pause of a tool call (`ai.InterruptWith`,
`FinishReasonInterrupted`, `RestartWith`/`RespondWith`). Sarza's chat flow defines **zero
tools today**, so there is nothing to handle yet and no silent gap: nothing in the flow
can emit `FinishReasonInterrupted`. The stop button is a different mechanism (transport
cancellation), implemented as above. When chat gains tools, the generate call must gain
the documented interrupt-resolution loop; noted here so it isn't forgotten.

## 2. Task Management — local Postgres

- `migrations/0002_task.sql`: `task(id, title, notes, status CHECK ('todo','doing','done'),
  priority, created_at, updated_at)`.
- New `internal/task` package (own `Store` interface + pgx adapter) rather than widening
  `secondbrain.Store` — widening would break every existing fake Store in
  api/mcp tests for no benefit. Same domain/adapter split as `secondbrain`.
- Routes: `GET/POST /tasks`, `PUT/DELETE /tasks/:id`. UI: `TasksView.vue`, three status
  columns, add/move/delete. No YouTrack (its deployment type is still unanswered — same
  standing blocker as Phase 8).

## 3. Knowledge Graph — derived from Second Brain

- New `internal/graph` package: `Build(ctx, pool, maxEdges)` — nodes are a UNION of
  `profile_rule` / `memory_entry` / `project_context` rows (kind, id, label, snippet);
  edges are pairwise pgvector cosine similarity over their embeddings above a threshold,
  `ORDER BY similarity DESC LIMIT maxEdges`. O(n²) in SQL — fine for a single-user desk
  DB; documented ceiling, revisit if node count grows past a few thousand.
- Route: `GET /graph`. UI: `KnowledgeGraphView.vue` — hand-rolled force-directed layout on
  SVG (no d3 dependency), node color per kind, click → detail panel. Static layout when
  `prefers-reduced-motion`.
- Rows with NULL embeddings appear as isolated nodes (edges require embeddings). Honest
  display, not hidden.

## 4/5. Skill Catalog + Prompt Catalog — filesystem, browse-only

- New `internal/library` package reading two directories: skills root (24 real skill dirs
  with `SKILL.md` YAML frontmatter `name:`/`description:`) and prompts root (`.prompt`
  dotprompt files). Minimal frontmatter parse (only `name`/`description` scalar lines) —
  no YAML dependency.
- Name parameter validated against the actual directory listing (no path joins with user
  input) — path-traversal guard.
- Routes: `GET /skills`, `GET /skills/:name`, `GET /prompts`, `GET /prompts/:name`.
- `cmd/core` resolves `skills/` the same way it already resolves `prompts/` (CWD then
  executable-sibling); `Makefile desktop-build` copies `skills/` into the bundle next to
  `prompts/`.
- UI: `SkillCatalogView.vue` / `PromptCatalogView.vue` — search filter, card list,
  detail pane rendering raw markdown text (`pre-wrap`, no md renderer dep). Browse-only;
  editing stays in the repo/editor, per user decision.

## 6. Voice Assistant — push-to-talk through Gemini multimodal

- WKWebView has no Web Speech *recognition* API — browser STT is out (that's why
  push-to-talk-through-Gemini won the scope question).
- `VoiceView.vue`: `getUserMedia` + `MediaRecorder` (mime negotiated via
  `isTypeSupported`, Safari/WKWebView yields `audio/mp4`), recorded blob → data URL →
  existing `POST /chat` with the audio as the media attachment (the chat flow's
  `dataURLMimeType`/`NewMediaPart` path is mime-agnostic — verified, it never assumes
  image/*) and a fixed instruction message. Gemini transcribes + answers in one call.
- Reply shown in a conversation list and spoken via `speechSynthesis` (supported in
  WKWebView), with a mute toggle.
- `cmd/desktop/build/darwin/Info.plist` (+ dev variant) gains
  `NSMicrophoneUsageDescription`. **Known verification gap:** mic capture inside the
  packaged Wails app can't be exercised in this sandbox (no mic, no GUI interaction) —
  needs the user's own click-test; the failure mode is a permission prompt that never
  appears, and the view surfaces `getUserMedia` errors visibly rather than failing silent.

## Sidebar

All five items move from the disabled `upcoming` group to the real `available` list;
the `Soon` badge pattern disappears from the sidebar (nothing left is fake).

## Non-goals (this pass)

- Gemini Live realtime voice (bidirectional audio streaming) — explicitly not chosen.
- Editable catalogs, user-curated graph tables, YouTrack tasks — not chosen.
- Persisting chat/voice conversations to `memory_entry` — separate feature, not asked.
- Markdown rendering of chat replies — out of scope here (plain text stays).
