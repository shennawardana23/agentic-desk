CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS profile_rule (
    id BIGSERIAL PRIMARY KEY,
    source_file TEXT NOT NULL,
    heading TEXT NOT NULL,
    line_start INT NOT NULL,
    line_end INT NOT NULL,
    content_hash TEXT NOT NULL,
    content TEXT NOT NULL,
    overridden BOOLEAN NOT NULL DEFAULT FALSE,
    embedding VECTOR(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_file, heading)
);

CREATE INDEX IF NOT EXISTS profile_rule_embedding_idx
    ON profile_rule USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS project_context (
    id BIGSERIAL PRIMARY KEY,
    project_path TEXT NOT NULL UNIQUE,
    summary TEXT NOT NULL DEFAULT '',
    embedding VECTOR(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS memory_entry (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'agent')),
    content TEXT NOT NULL,
    embedding VECTOR(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS memory_entry_embedding_idx
    ON memory_entry USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS feedback_signal (
    id BIGSERIAL PRIMARY KEY,
    memory_entry_id BIGINT REFERENCES memory_entry(id) ON DELETE SET NULL,
    decision TEXT NOT NULL CHECK (decision IN ('approve', 'correct', 'reject')),
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
