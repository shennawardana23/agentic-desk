package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/middleware"

	"github.com/shennawardana23/agentic-desk/internal/provider/chain"
)

// ChatFlowName is the name Sarza's conversational flow is registered under.
const ChatFlowName = "chatFlow"

// ChatStreamFlowName is the streaming variant's registration name.
const ChatStreamFlowName = "chatStreamFlow"

// SarzaSystemPrompt is Sarza's persona instruction. Kept short and specific
// to this app rather than a generic "you are a helpful assistant" — Sarza is
// scoped to what this app can actually do (Second Brain profile/memory/project
// context), not a general-purpose chatbot, so it doesn't imply capabilities
// (browsing, code execution, etc.) that don't exist here.
const SarzaSystemPrompt = "You are Sarza, the conversational agent inside Agentic Desk — a developer's personal Second Brain. " +
	"You help the user think through their coding profile rules, project context, and session memory. " +
	"Be direct and concise. If asked to do something outside a text conversation (browse the web, run code, edit files), " +
	"say plainly that you can't do that yet rather than pretending to."

// ChatTurn is one message in a conversation, in the wire format the
// frontend/API layer uses (see internal/api's /chat route and
// stores/core.js's sendChatMessage, which is this format's other end).
type ChatTurn struct {
	Role    string `json:"role"` // "user" or "agent"
	Content string `json:"content"`
	// ImageDataURL, if set, is a data: URL (e.g. "data:image/png;base64,...")
	// attached to a user turn. Only meaningful when Role == "user".
	ImageDataURL string `json:"imageDataUrl,omitempty"`
}

// ChatInput is DefineChatFlow's input: the prior turns plus the new message.
// History is omitempty so a first-turn call may omit it entirely — the
// streaming flow path (unlike Flow.Run) enforces the inferred JSON schema
// on input, and a required "history" would reject nil (caught by
// TestChatStreamFlow_ForwardsReasoningAndTextChunks when this tag was absent).
type ChatInput struct {
	History []ChatTurn `json:"history,omitempty"`
	Message string     `json:"message"`
	// ImageDataURL attaches an image to Message itself — see ChatTurn's field
	// of the same name. ai.NewMediaPart is the real Genkit Go API for sending
	// inline image data to a multimodal model (verified against the pinned
	// v1.10.0 source, github.com/firebase/genkit/go/ai/document.go).
	ImageDataURL string `json:"imageDataUrl,omitempty"`
}

// ChatOutput is DefineChatFlow's output.
type ChatOutput struct {
	Reply string `json:"reply"`
}

// ChatChunk is one streamed increment from DefineChatStreamFlow: either a
// piece of the model's own reasoning (Gemini "thoughts", surfaced as real
// ai.PartReasoning parts by the googlegenai plugin when
// thinkingConfig.includeThoughts is set — verified against the pinned
// v1.10.0 source, plugins/googlegenai/gemini.go) or a piece of the visible
// reply text. The GUI renders reasoning chunks in the collapsible
// "Thinking" section and text chunks in the reply bubble.
type ChatChunk struct {
	Type    string `json:"type"` // "reasoning" or "text"
	Content string `json:"content"`
}

// dataURLMimeType extracts "<mime>" from a "data:<mime>;base64,..." URL.
// ai.NewMediaPart(mimeType, contents string) sets mimeType verbatim as
// the Part's ContentType (verified against ai/document.go — it does NOT
// parse contents itself), so passing "" there — the wire format's
// original bug — would send Gemini a media part with no content type,
// silently breaking real image input. Returns "" if dataURL isn't a
// well-formed data: URL, so the caller can skip the media part rather
// than send a mimeType Gemini would reject anyway.
func dataURLMimeType(dataURL string) string {
	const prefix = "data:"
	if !strings.HasPrefix(dataURL, prefix) {
		return ""
	}
	rest := dataURL[len(prefix):]
	if semi := strings.IndexByte(rest, ';'); semi >= 0 {
		return rest[:semi]
	}
	return ""
}

// newUserMessage builds a single user turn, threading an attached image (as
// a real ai.NewMediaPart, verified against the pinned v1.10.0 source at
// github.com/firebase/genkit/go/ai/document.go) alongside its text.
func newUserMessage(text, imageDataURL string) *ai.Message {
	parts := []*ai.Part{ai.NewTextPart(text)}
	if mime := dataURLMimeType(imageDataURL); mime != "" {
		parts = append(parts, ai.NewMediaPart(mime, imageDataURL))
	}
	return ai.NewUserMessage(parts...)
}

// toGenkitMessages converts wire-format history into Genkit's *ai.Message
// history, preserving order and role, and threading any attached image into
// its turn's parts alongside the text.
func toGenkitMessages(history []ChatTurn) []*ai.Message {
	msgs := make([]*ai.Message, 0, len(history))
	for _, t := range history {
		if t.Role == "agent" {
			msgs = append(msgs, ai.NewModelTextMessage(t.Content))
			continue
		}
		msgs = append(msgs, newUserMessage(t.Content, t.ImageDataURL))
	}
	return msgs
}

// DefineChatFlow registers Sarza's flows (request/response + streaming,
// via DefineChatFlows) and returns the request/response one — kept for
// callers that only need that half. Real generation across a
// multi-provider chain (see internal/provider/chain) — Gemini
// (PrimaryModel/FallbackModel) first, then any other provider whose
// API-key env var is set, in the order chain.Build assembles — with
// the same Fallback(outer)+Retry(inner) resilience stack
// DefinePlaceholderFlow already proved works end-to-end. chain.Build
// runs once, at flow-definition time (compat_oai's Init only sets
// up an HTTP client, no network call), not per request. If it somehow
// returns no models at all, the flows still get defined but every call
// fails with a clear error rather than the whole app failing to start.
// The new message is appended as the final entry in ai.WithMessages
// rather than passed via ai.WithPrompt (which is text-only — no
// WithPromptParts/media variant exists in this pinned SDK version,
// verified against ai/option.go) — WithMessages' documented
// "sandwiched between system and user prompts" behavior means the full
// message list, ending in the new turn, IS the user prompt here.
func DefineChatFlow(g *genkit.Genkit) *core.Flow[ChatInput, ChatOutput, struct{}] {
	flows := DefineChatFlows(g)
	return flows.Chat
}

// ChatFlows bundles the two registrations DefineChatFlows makes over one
// shared provider chain.
type ChatFlows struct {
	Chat   *core.Flow[ChatInput, ChatOutput, struct{}]
	Stream *core.Flow[ChatInput, ChatOutput, ChatChunk]
}

// DefineChatFlows registers both the request/response chat flow and its
// streaming sibling over a single chain.Build call — building the chain
// twice would call each joined provider's DefineModel twice, and Genkit's
// registry rejects duplicate action registration (the exact failure mode
// iteration 6's nested-prompts bug already demonstrated for prompts).
func DefineChatFlows(g *genkit.Genkit) ChatFlows {
	models, chainErr := chain.Build(context.Background(), g, PrimaryModel, FallbackModel)
	if chainErr != nil {
		slog.Error("chat flow: provider chain build failed, chat will error on every call", "err", chainErr)
	}

	generate := func(ctx context.Context, in ChatInput, send core.StreamCallback[ChatChunk]) (ChatOutput, error) {
		if chainErr != nil {
			return ChatOutput{}, fmt.Errorf("chat flow: no model provider available: %w", chainErr)
		}
		if strings.TrimSpace(in.Message) == "" {
			return ChatOutput{}, fmt.Errorf("chat flow: message is required")
		}

		messages := append(toGenkitMessages(in.History), newUserMessage(in.Message, in.ImageDataURL))

		opts := []ai.GenerateOption{
			ai.WithModel(models[0]),
			ai.WithSystem(SarzaSystemPrompt),
			ai.WithMessages(messages...),
			// Ask Gemini to surface its internal reasoning as real
			// ai.PartReasoning parts (plugins/googlegenai/gemini.go converts
			// thought parts when thinkingConfig.includeThoughts is set —
			// verified against the pinned v1.10.0 source). Passed as a map:
			// googlegenai decodes map configs into genai.GenerateContentConfig
			// (gemini.go configFromRequest), while compat_oai's map path
			// json-round-trips into openai params where this unknown key is
			// silently dropped — so the Fallback chain's non-Gemini providers
			// keep working, they just never emit reasoning chunks.
			ai.WithConfig(map[string]any{
				"thinkingConfig": map[string]any{"includeThoughts": true},
			}),
			ai.WithUse(
				&middleware.Fallback{Models: chain.Refs(models)},
				&middleware.Retry{MaxRetries: 3},
			),
		}
		if send != nil {
			opts = append(opts, ai.WithStreaming(func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
				for _, p := range chunk.Content {
					var kind string
					switch {
					case p.IsReasoning():
						kind = "reasoning"
					case p.IsText():
						kind = "text"
					default:
						continue
					}
					if p.Text == "" {
						continue
					}
					if err := send(ctx, ChatChunk{Type: kind, Content: p.Text}); err != nil {
						return err
					}
				}
				return nil
			}))
		}

		resp, err := genkit.Generate(ctx, g, opts...)
		if err != nil {
			return ChatOutput{}, fmt.Errorf("chat flow: %w", err)
		}
		return ChatOutput{Reply: resp.Text()}, nil
	}

	return ChatFlows{
		Chat: genkit.DefineFlow(g, ChatFlowName, func(ctx context.Context, in ChatInput) (ChatOutput, error) {
			return generate(ctx, in, nil)
		}),
		Stream: genkit.DefineStreamingFlow(g, ChatStreamFlowName, generate),
	}
}
