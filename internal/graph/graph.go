// Package graph derives the Knowledge Graph view from Second Brain data:
// nodes are the existing profile rules / memory entries / project contexts,
// edges are pgvector cosine similarity between their embeddings. No new
// writes, no curated graph tables — the graph is a projection of what the
// desk already knows (design doc 2026-07-07 §3).
package graph

import "context"

// Node is one Second Brain record projected into the graph.
type Node struct {
	ID      string `json:"id"`   // "<kind>:<row id>", e.g. "memory:12"
	Kind    string `json:"kind"` // "rule" | "memory" | "project"
	Label   string `json:"label"`
	Snippet string `json:"snippet,omitempty"`
}

// Edge links two nodes whose embeddings are cosine-similar.
type Edge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Weight float64 `json:"weight"` // cosine similarity, 0..1
}

// Data is what GET /graph returns.
type Data struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Builder is the port the API layer depends on; postgres.Builder implements it.
type Builder interface {
	Build(ctx context.Context) (Data, error)
}
