// Package postgres is internal/task's sole pgx-importing adapter, same
// layering as internal/secondbrain/postgres.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shennawardana23/agentic-desk/internal/task"
)

// Store implements task.Store against the pool cmd/core already opens.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps an existing pool; it does not own its lifecycle.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const taskColumns = "id, title, notes, description, status, priority, created_at, updated_at"

func scanTask(row pgx.Row) (task.Task, error) {
	var t task.Task
	err := row.Scan(&t.ID, &t.Title, &t.Notes, &t.Description, &t.Status, &t.Priority, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (s *Store) Create(ctx context.Context, t task.Task) (task.Task, error) {
	if t.Status == "" {
		t.Status = task.StatusTodo
	}
	if err := t.Validate(); err != nil {
		return task.Task{}, err
	}
	created, err := scanTask(s.pool.QueryRow(ctx,
		"INSERT INTO task (title, notes, description, status, priority) VALUES ($1, $2, $3, $4, $5) RETURNING "+taskColumns,
		t.Title, t.Notes, t.Description, t.Status, t.Priority))
	if err != nil {
		return task.Task{}, fmt.Errorf("create task: %w", err)
	}
	return created, nil
}

func (s *Store) List(ctx context.Context) ([]task.Task, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+taskColumns+" FROM task ORDER BY priority DESC, created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	var tasks []task.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("list tasks: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) Update(ctx context.Context, t task.Task) (task.Task, error) {
	if err := t.Validate(); err != nil {
		return task.Task{}, err
	}
	updated, err := scanTask(s.pool.QueryRow(ctx,
		"UPDATE task SET title = $2, notes = $3, description = $4, status = $5, priority = $6, updated_at = now() WHERE id = $1 RETURNING "+taskColumns,
		t.ID, t.Title, t.Notes, t.Description, t.Status, t.Priority))
	if errors.Is(err, pgx.ErrNoRows) {
		return task.Task{}, task.ErrNotFound
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("update task: %w", err)
	}
	return updated, nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM task WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return task.ErrNotFound
	}
	return nil
}
