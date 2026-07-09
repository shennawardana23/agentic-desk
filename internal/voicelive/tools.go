package voicelive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/genai"
)

// BuildLiveTools returns Tool declarations for Gemini Live sessions.
// Design: ONE combined "creative_tool" instead of 5 separate ones —
// reduces per-turn evaluation overhead from ~500ms to ~100ms.
func BuildLiveTools() []*genai.Tool {
	return []*genai.Tool{
		// Google Search — native, zero evaluation overhead
		{GoogleSearch: &genai.GoogleSearch{}},

		// ONE combined tool for all creative/utility tasks
		{FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "creative_tool",
				Description: "Execute a creative or utility task. Types: 'image' (generate image), 'code' (write code), 'music' (music production brief), 'video' (video production brief), 'fetch' (fetch URL content). Use when user asks to create, generate, write, draw, compose, or fetch.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"type":     {Type: genai.TypeString, Description: "Task type: image, code, music, video, fetch"},
						"prompt":   {Type: genai.TypeString, Description: "Main description, task details, or URL for fetch"},
						"language": {Type: genai.TypeString, Description: "For code: programming language (go, python, js, etc.)"},
					},
					Required: []string{"type", "prompt"},
				},
			},
		}},
	}
}

// ToolExecutor handles tool execution with a shared HTTP client and genai client.
type ToolExecutor struct {
	http   *http.Client
	genai  *genai.Client
}

func newToolExecutor() *ToolExecutor {
	apiKey := os.Getenv("GEMINI_API_KEY")
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		slog.Error("voicelive tools: failed to create genai client", "err", err)
	}
	return &ToolExecutor{
		http:  &http.Client{Timeout: 15 * time.Second},
		genai: client,
	}
}

// Execute dispatches a FunctionCall to the correct handler.
func (te *ToolExecutor) Execute(ctx context.Context, fc *genai.FunctionCall) (map[string]any, error) {
	args := fc.Args
	if args == nil {
		args = map[string]any{}
	}

	taskType, _ := args["type"].(string)
	switch taskType {
	case "image":
		return te.genImage(ctx, args)
	case "code":
		return te.genText(ctx, args, "code")
	case "music":
		return te.genText(ctx, args, "music")
	case "video":
		return te.genText(ctx, args, "video")
	case "fetch":
		args["url"] = args["prompt"]
		return te.fetchURL(ctx, args)
	default:
		return map[string]any{"error": fmt.Sprintf("unknown task type: %q", taskType)}, nil
	}
}

// handleToolCalls runs all tool calls concurrently, sends results to browser,
// then sends a lightweight response back to Gemini (no binary data).
func handleToolCalls(ctx context.Context, gs liveSession, tc *genai.LiveServerToolCall, exec *ToolExecutor, outCh chan outMsg) {
	if len(tc.FunctionCalls) == 0 {
		return
	}

	type result struct {
		id, name string
		output   map[string]any
	}

	results := make([]result, len(tc.FunctionCalls))
	done := make(chan int, len(tc.FunctionCalls))

	for i, fc := range tc.FunctionCalls {
		go func(idx int, fc *genai.FunctionCall) {
			out, err := exec.Execute(ctx, fc)
			if err != nil {
				out = map[string]any{"error": err.Error()}
			}
			results[idx] = result{id: fc.ID, name: fc.Name, output: out}
			done <- idx
		}(i, fc)
	}
	for range tc.FunctionCalls {
		<-done
	}

	// Send FULL results to browser (including image_base64 for rendering).
	for _, r := range results {
		data, _ := json.Marshal(WSMessage{
			Type: WSTypeToolResult,
			Payload: map[string]interface{}{"name": r.name, "output": r.output},
		})
		select {
		case outCh <- outMsg{data: data}:
		default:
		}
		slog.Info("voicelive tool done", "name", r.name)
	}

	// Send LIGHTWEIGHT results to Gemini — strip large binary fields.
	// Gemini only needs a text summary to speak; it rejects raw image bytes.
	responses := make([]*genai.FunctionResponse, 0, len(results))
	for _, r := range results {
		light := make(map[string]any)
		for k, v := range r.output {
			if k == "image_base64" || k == "image_data" {
				continue
			}
			light[k] = v
		}
		j, _ := json.Marshal(light)
		responses = append(responses, &genai.FunctionResponse{
			ID: r.id, Name: r.name,
			Response: map[string]any{"result": string(j)},
		})
	}

	var mu sync.Mutex
	mu.Lock()
	err := gs.SendToolResponse(genai.LiveSendToolResponseParameters{FunctionResponses: responses})
	mu.Unlock()
	if err != nil {
		slog.Error("voicelive: SendToolResponse failed", "err", err)
	}
}

// ─── fetch_url ───────────────────────────────────────────────────────────────

func (te *ToolExecutor) fetchURL(ctx context.Context, args map[string]any) (map[string]any, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return map[string]any{"error": "url required"}, nil
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	req.Header.Set("User-Agent", "AgenticDesk/1.0")
	resp, err := te.http.Do(req)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	return map[string]any{"text": trunc(stripHTML(string(body)), 4000)}, nil
}

// ─── generate_image ──────────────────────────────────────────────────────────

func (te *ToolExecutor) genImage(ctx context.Context, args map[string]any) (map[string]any, error) {
	prompt, _ := args["prompt"].(string)
	if prompt == "" || te.genai == nil {
		return map[string]any{"error": "prompt required or genai client unavailable"}, nil
	}
	slog.Info("voicelive tool: generate_image", "prompt", trunc(prompt, 60))

	result, err := te.genai.Models.GenerateContent(ctx,
		"gemini-2.5-flash-image",
		[]*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)},
		&genai.GenerateContentConfig{
			ResponseModalities: []string{"IMAGE", "TEXT"},
		},
	)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			return map[string]any{
				"status":        "success",
				"image_base64":  "data:" + part.InlineData.MIMEType + ";base64," + string(part.InlineData.Data),
				"image_mime":    part.InlineData.MIMEType,
				"message":       "Image generated successfully",
			}, nil
		}
	}
	return map[string]any{"error": "no image in response"}, nil
}

// ─── generate_text (code / music / video) ────────────────────────────────────

func (te *ToolExecutor) genText(ctx context.Context, args map[string]any, kind string) (map[string]any, error) {
	prompt, _ := args["prompt"].(string)
	if prompt == "" || te.genai == nil {
		return map[string]any{"error": "prompt required or genai client unavailable"}, nil
	}

	var sysPrompt string
	switch kind {
	case "code":
		lang, _ := args["language"].(string)
		if lang == "" {
			lang = "the most appropriate language"
		}
		sysPrompt = fmt.Sprintf("Generate clean, production-ready %s code for: %s\n\nReturn only the code with brief comments. No markdown fences.", lang, prompt)
	case "music":
		sysPrompt = fmt.Sprintf("Create a detailed music production brief for: %s\n\nInclude: Title, Genre, Tempo (BPM), Key, Instruments, Song Structure, Production Notes. Format as a structured brief.", prompt)
	case "video":
		sysPrompt = fmt.Sprintf("Create a video production brief for: %s\n\nInclude: Visual Style, Shot List (5-8 shots with camera movements), Audio/Music direction, Estimated duration. Format as a structured brief.", prompt)
	}

	slog.Info("voicelive tool: gen_"+kind, "prompt", trunc(prompt, 60))
	result, err := te.genai.Models.GenerateContent(ctx,
		"gemini-2.0-flash",
		[]*genai.Content{genai.NewContentFromText(sysPrompt, genai.RoleUser)},
		nil,
	)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	content := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, p := range result.Candidates[0].Content.Parts {
			if p.Text != "" {
				content += p.Text
			}
		}
	}
	return map[string]any{"status": "success", "content": content, "type": kind}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	// Collapse whitespace
	out := strings.Join(strings.Fields(b.String()), " ")
	return out
}
