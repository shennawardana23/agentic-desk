// Package adkbridge translates a Genkit ai.Model into ADK's model.LLM
// interface — forward-looking plumbing for the still-deferred Phase 6b
// agent runtime (PLAN.md). Nothing in this repo constructs or calls it
// yet: agentic-desk's chat flow calls genkit.Generate/ai.WithModel
// directly and needs no ADK translation to *use* a model.
//
// Unlike the reference implementation this design was scoped from
// (archpublicwebsite-agentic/internal/model/oaibridge), this package
// does not construct providers itself — internal/provider/* already
// builds every ai.Model this repo needs, including Fallback/Retry
// middleware composition at the Genkit layer. adkbridge's only job is
// the ADK↔Genkit type translation around an already-built ai.Model.
// The reference's per-provider concurrency semaphore is deliberately
// not ported here either — Retry already degrades 429s gracefully and
// this is a single-user desktop app; add one if Phase 6b ever runs
// multiple ADK agents concurrently against a low-limit provider.
//
// Known limitation, inherited by being more generic than the
// reference: genaiConfigToMap produces a loose map[string]any for
// ai.ModelRequest.Config. compat_oai-backed models (the reference's
// only target) interpret that map; a native plugin like googlegenai or
// anthropic expects its own typed config struct instead, so bridging a
// non-compat_oai ai.Model may silently drop temperature/max_tokens/etc.
// Not fixed here — no consumer exists yet to prove which shape Phase
// 6b actually needs.
//
// # Type mapping (ADK google.golang.org/genai ↔ Genkit ai)
//
//	genai.Content{Role:"user"}          → ai.Message{Role: ai.RoleUser}
//	genai.Content{Role:"user"}+FuncResp → ai.Message{Role: ai.RoleTool}
//	genai.Content{Role:"model"}         → ai.Message{Role: ai.RoleModel}
//	Config.SystemInstruction            → ai.Message{Role: ai.RoleSystem} (prepended)
//	genai.Part{Text}                    → ai.NewTextPart(...)
//	genai.Part{FunctionCall}            → ai.NewToolRequestPart(...)
//	genai.Part{FunctionResponse}        → ai.NewToolResponsePart(...)
//
// Confirmed against the pinned google.golang.org/adk@v1.5.0 and
// google.golang.org/genai@v1.57.0 module sources directly — not
// assumed from the reference repo's (older) versions.
package adkbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/firebase/genkit/go/ai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type llmBridge struct {
	name    string
	aiModel ai.Model
}

// NewModel wraps an existing ai.Model as an ADK model.LLM.
func NewModel(m ai.Model) (model.LLM, error) {
	if m == nil {
		return nil, fmt.Errorf("adkbridge.NewModel: m is required")
	}
	return &llmBridge{name: m.Name(), aiModel: m}, nil
}

// Name satisfies model.LLM.
func (b *llmBridge) Name() string { return b.name }

// GenerateContent satisfies model.LLM. Streaming is not implemented —
// stream is accepted for interface compatibility but always yields a
// single complete response, matching this repo's current non-streaming
// chat flow; revisit if/when Phase 6b needs token-level streaming.
func (b *llmBridge) GenerateContent(
	ctx context.Context,
	req *model.LLMRequest,
	_ bool,
) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		aiReq, err := toAIRequest(req)
		if err != nil {
			yield(nil, fmt.Errorf("adkbridge: build request: %w", err))
			return
		}
		resp, err := b.aiModel.Generate(ctx, aiReq, nil)
		if err != nil {
			yield(nil, fmt.Errorf("adkbridge: generate [%s]: %w", b.name, err))
			return
		}
		yield(fromAIResponse(resp), nil)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Request conversion: ADK (genai) → Genkit (ai)
// ─────────────────────────────────────────────────────────────────────

func toAIRequest(req *model.LLMRequest) (*ai.ModelRequest, error) {
	aiReq := &ai.ModelRequest{}

	if req.Config != nil && req.Config.SystemInstruction != nil {
		sysMsg, err := contentToMessage(req.Config.SystemInstruction, ai.RoleSystem)
		if err != nil {
			return nil, fmt.Errorf("system instruction: %w", err)
		}
		aiReq.Messages = append(aiReq.Messages, sysMsg)
	}

	for i, c := range req.Contents {
		msg, err := contentToMessage(c, "")
		if err != nil {
			return nil, fmt.Errorf("contents[%d]: %w", i, err)
		}
		aiReq.Messages = append(aiReq.Messages, msg)
	}

	if req.Config != nil {
		for _, gt := range req.Config.Tools {
			for _, decl := range gt.FunctionDeclarations {
				td, err := declToToolDef(decl)
				if err != nil {
					return nil, fmt.Errorf("tool %q: %w", decl.Name, err)
				}
				aiReq.Tools = append(aiReq.Tools, td)
			}
		}
		if cfg := genaiConfigToMap(req.Config); cfg != nil {
			aiReq.Config = cfg
		}
	}
	return aiReq, nil
}

func contentToMessage(c *genai.Content, role ai.Role) (*ai.Message, error) {
	r := role
	if r == "" {
		switch c.Role {
		case "user":
			for _, p := range c.Parts {
				if p.FunctionResponse != nil {
					r = ai.RoleTool
					break
				}
			}
			if r == "" {
				r = ai.RoleUser
			}
		case "model":
			r = ai.RoleModel
		default:
			r = ai.Role(c.Role)
		}
	}

	msg := &ai.Message{Role: r}
	for _, p := range c.Parts {
		ap, err := genaiPartToAI(p)
		if err != nil {
			return nil, err
		}
		if ap != nil {
			msg.Content = append(msg.Content, ap)
		}
	}
	return msg, nil
}

func genaiPartToAI(p *genai.Part) (*ai.Part, error) {
	switch {
	case p.Text != "":
		return ai.NewTextPart(p.Text), nil
	case p.FunctionCall != nil:
		fc := p.FunctionCall
		return ai.NewToolRequestPart(&ai.ToolRequest{
			Name:  fc.Name,
			Input: fc.Args,
			Ref:   fc.ID,
		}), nil
	case p.FunctionResponse != nil:
		fr := p.FunctionResponse
		return ai.NewToolResponsePart(&ai.ToolResponse{
			Name:   fr.Name,
			Output: fr.Response,
			Ref:    fr.ID,
		}), nil
	default:
		return nil, nil
	}
}

func declToToolDef(decl *genai.FunctionDeclaration) (*ai.ToolDefinition, error) {
	schema, err := anyToMap(decl.ParametersJsonSchema)
	if err != nil {
		return nil, fmt.Errorf("parameters schema: %w", err)
	}
	return &ai.ToolDefinition{
		Name:        decl.Name,
		Description: decl.Description,
		InputSchema: schema,
	}, nil
}

func genaiConfigToMap(c *genai.GenerateContentConfig) map[string]any {
	if c == nil {
		return nil
	}
	m := make(map[string]any, 4)
	if c.Temperature != nil {
		m["temperature"] = float64(*c.Temperature)
	}
	if c.MaxOutputTokens != 0 {
		m["max_tokens"] = int(c.MaxOutputTokens)
	}
	if c.TopP != nil {
		m["top_p"] = float64(*c.TopP)
	}
	if len(c.StopSequences) > 0 {
		m["stop"] = c.StopSequences
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// ─────────────────────────────────────────────────────────────────────
// Response conversion: Genkit (ai) → ADK (genai)
// ─────────────────────────────────────────────────────────────────────

func fromAIResponse(resp *ai.ModelResponse) *model.LLMResponse {
	if resp == nil || resp.Message == nil {
		return &model.LLMResponse{TurnComplete: true}
	}
	content := &genai.Content{Role: "model"}
	for _, p := range resp.Message.Content {
		gp := aiPartToGenai(p)
		if gp != nil {
			content.Parts = append(content.Parts, gp)
		}
	}
	llmResp := &model.LLMResponse{Content: content, TurnComplete: true}
	if resp.Usage != nil {
		// #nosec G115 -- token counts are bounded by provider context-window
		// limits (low millions at most), many orders of magnitude below
		// int32 max.
		llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.InputTokens),
			CandidatesTokenCount: int32(resp.Usage.OutputTokens),
			TotalTokenCount:      int32(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		}
	}
	return llmResp
}

func aiPartToGenai(p *ai.Part) *genai.Part {
	switch p.Kind {
	case ai.PartText:
		return &genai.Part{Text: p.Text}
	case ai.PartToolRequest:
		if p.ToolRequest == nil {
			return nil
		}
		args, _ := anyToMap(p.ToolRequest.Input)
		return &genai.Part{FunctionCall: &genai.FunctionCall{
			ID:   p.ToolRequest.Ref,
			Name: p.ToolRequest.Name,
			Args: args,
		}}
	default:
		return nil
	}
}

// ─────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────

func anyToMap(v any) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return m, nil
}
