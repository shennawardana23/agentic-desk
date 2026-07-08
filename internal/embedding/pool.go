package embedding

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// Job is one unit of work: an opaque ID paired with the text to embed.
// The pool doesn't interpret ID — callers (the importer, Second Brain
// writers, etc.) decide what it refers to and what to do with the
// resulting embedding in onResult.
type Job struct {
	ID   int64
	Text string
}

// Result is what Run reports for each completed Job — exactly one of
// Embedding or Err is set.
type Result struct {
	ID        int64
	Embedding []float32
	Err       error
}

// Pool runs a fixed number of worker goroutines pulling Jobs off a
// channel and calling Embedder.Embed, bounding how many embedding
// requests are in flight at once regardless of how many Jobs exist.
type Pool struct {
	embedder Embedder
	workers  int
}

// NewPool constructs a Pool. workers <= 0 is treated as 1 — a pool
// that runs nothing isn't useful.
func NewPool(embedder Embedder, workers int) *Pool {
	if workers <= 0 {
		workers = 1
	}
	return &Pool{embedder: embedder, workers: workers}
}

// Run embeds every job in jobs, calling onResult once per job (from a
// worker goroutine — onResult must be safe for concurrent use). It
// stops launching new jobs as soon as ctx is canceled, then waits up
// to shutdownTimeout for jobs already in flight to finish before
// returning — a bounded shutdown instead of blocking forever on a
// stuck or slow request. (errgroup.Group's own WaitGroup, waited on
// through a timeout select below, plays the "WaitGroup.Wait(timeout)"
// role — Go's stdlib sync.WaitGroup has no built-in timeout, so this
// is the idiomatic way to bound it without hand-rolling one.)
func (p *Pool) Run(ctx context.Context, jobs []Job, onResult func(Result), shutdownTimeout time.Duration) error {
	jobCh := make(chan Job)
	g, gctx := errgroup.WithContext(ctx)

	for i := 0; i < p.workers; i++ {
		g.Go(func() error {
			for job := range jobCh {
				embedding, err := p.embedder.Embed(gctx, job.Text)
				onResult(Result{ID: job.ID, Embedding: embedding, Err: err})
			}
			return nil
		})
	}

feed:
	for _, job := range jobs {
		select {
		case jobCh <- job:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobCh)

	waitErr := make(chan error, 1)
	go func() { waitErr <- g.Wait() }()

	select {
	case err := <-waitErr:
		return err
	case <-time.After(shutdownTimeout):
		return fmt.Errorf("embedding pool: shutdown timed out after %s with jobs still in flight", shutdownTimeout)
	}
}
