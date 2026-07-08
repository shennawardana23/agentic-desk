-- Task Management: local, single-user task board (design doc
-- 2026-07-07-chat-streaming-and-domain-features-design.md §2).
-- Deliberately not partitioned and not YouTrack-backed — personal desk
-- data, one user, local Postgres.
CREATE TABLE IF NOT EXISTS task (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'doing', 'done')),
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS task_status_idx ON task (status);
