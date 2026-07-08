// Package postgres is internal/graph's sole pgx-importing adapter, same
// layering as internal/secondbrain/postgres and internal/task/postgres.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shennawardana23/agentic-desk/internal/graph"
)

// Builder implements graph.Builder against the pool cmd/core already opens.
type Builder struct {
	pool     *pgxpool.Pool
	maxNodes int
	maxEdges int
	minSim   float64
}

// NewBuilder wraps an existing pool. Defaults: 500 nodes, 300 edges,
// similarity floor 0.55 — sized for a personal desk DB, not a warehouse.
func NewBuilder(pool *pgxpool.Pool) *Builder {
	return &Builder{pool: pool, maxNodes: 500, maxEdges: 300, minSim: 0.55}
}

// nodeCTE projects the three Second Brain tables into one (id, kind,
// label, snippet, embedding) shape. Labels/snippets are truncated
// server-side — the GUI never needs full bodies for the graph canvas.
const nodeCTE = `
WITH node AS (
    SELECT 'rule:' || id AS id, 'rule' AS kind, heading AS label,
           left(content, 200) AS snippet, embedding, created_at
    FROM profile_rule
    UNION ALL
    SELECT 'memory:' || id, 'memory', left(content, 80),
           left(content, 200), embedding, created_at
    FROM memory_entry
    UNION ALL
    SELECT 'project:' || id, 'project', project_path,
           left(summary, 200), embedding, created_at
    FROM project_context
)`

func (b *Builder) Build(ctx context.Context) (graph.Data, error) {
	data := graph.Data{Nodes: []graph.Node{}, Edges: []graph.Edge{}}

	rows, err := b.pool.Query(ctx,
		nodeCTE+` SELECT id, kind, label, snippet FROM node ORDER BY created_at DESC LIMIT $1`,
		b.maxNodes)
	if err != nil {
		return graph.Data{}, fmt.Errorf("graph nodes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n graph.Node
		if err := rows.Scan(&n.ID, &n.Kind, &n.Label, &n.Snippet); err != nil {
			return graph.Data{}, fmt.Errorf("graph nodes: %w", err)
		}
		data.Nodes = append(data.Nodes, n)
	}
	if err := rows.Err(); err != nil {
		return graph.Data{}, fmt.Errorf("graph nodes: %w", err)
	}

	// Pairwise cosine similarity across every embedded node — O(n²) in
	// SQL, bounded by maxNodes above. ponytail: fine below a few thousand
	// nodes; if the desk ever outgrows that, switch to per-node HNSW
	// k-NN queries (the indexes already exist).
	edgeRows, err := b.pool.Query(ctx,
		nodeCTE+`, embedded AS (
		    SELECT id, embedding FROM node WHERE embedding IS NOT NULL
		)
		SELECT a.id, b.id, 1 - (a.embedding <=> b.embedding) AS weight
		FROM embedded a JOIN embedded b ON a.id < b.id
		WHERE 1 - (a.embedding <=> b.embedding) >= $1
		ORDER BY weight DESC
		LIMIT $2`,
		b.minSim, b.maxEdges)
	if err != nil {
		return graph.Data{}, fmt.Errorf("graph edges: %w", err)
	}
	defer edgeRows.Close()
	for edgeRows.Next() {
		var e graph.Edge
		if err := edgeRows.Scan(&e.Source, &e.Target, &e.Weight); err != nil {
			return graph.Data{}, fmt.Errorf("graph edges: %w", err)
		}
		data.Edges = append(data.Edges, e)
	}
	if err := edgeRows.Err(); err != nil {
		return graph.Data{}, fmt.Errorf("graph edges: %w", err)
	}
	return data, nil
}
