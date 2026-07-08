// Package task is the Task Management domain: a local, single-user task
// board backed by Postgres (migrations/0002_task.sql). Same domain/adapter
// split as internal/secondbrain — this file is driver-free, postgres/ is
// the only pgx importer. A separate package (not a widening of
// secondbrain.Store) so the existing fakes in api/mcp tests stay untouched.
package task

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound mirrors secondbrain.ErrNotFound's role for this domain.
var ErrNotFound = errors.New("task not found")

// Statuses a task may be in — mirrors 0002_task.sql's CHECK constraint.
const (
	StatusTodo  = "todo"
	StatusDoing = "doing"
	StatusDone  = "done"
)

// Task is one row on the board.
type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Notes       string    `json:"notes"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Validate mirrors the DB's own constraints so a bad write fails with a
// clear domain error before touching the database.
func (t Task) Validate() error {
	if t.Title == "" {
		return fmt.Errorf("task: title is required")
	}
	switch t.Status {
	case StatusTodo, StatusDoing, StatusDone:
		return nil
	default:
		return fmt.Errorf("task: invalid status %q", t.Status)
	}
}

// Store is the port the API layer depends on; postgres.Store implements it.
type Store interface {
	Create(ctx context.Context, t Task) (Task, error)
	List(ctx context.Context) ([]Task, error)
	Update(ctx context.Context, t Task) (Task, error)
	Delete(ctx context.Context, id int64) error
}
