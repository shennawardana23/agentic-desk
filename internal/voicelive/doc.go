// Package voicelive implements the Gemini Live API voice agent backend.
// Architecture mirrors archpublicwebsite-mcp/internal/service/agentlive:
//   - Service: session lifecycle, model catalog, preset management
//   - Session: bidirectional Gemini bridge with outCh buffering
//   - Tools: GoogleSearch + creative_tool (image/code/music/video/fetch)
//   - Types: domain entities, WS protocol constants
package voicelive
