package embedding_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/shennawardana23/agentic-desk/internal/embedding"
)

// fakeEmbedder returns a deterministic vector after delay, tracking the
// maximum number of concurrent Embed calls it observed. It respects
// context cancellation, like a well-behaved real embedder would.
type fakeEmbedder struct {
	delay time.Duration

	mu            sync.Mutex
	concurrent    int
	maxConcurrent int
	calls         int
}

func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	f.mu.Lock()
	f.concurrent++
	f.calls++
	if f.concurrent > f.maxConcurrent {
		f.maxConcurrent = f.concurrent
	}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.concurrent--
		f.mu.Unlock()
	}()

	select {
	case <-time.After(f.delay):
		return []float32{float32(len(text))}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *fakeEmbedder) maxObservedConcurrency() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxConcurrent
}

func collectResults(pool *embedding.Pool, ctx context.Context, jobs []embedding.Job, timeout time.Duration) ([]embedding.Result, error) {
	var mu sync.Mutex
	var results []embedding.Result
	err := pool.Run(ctx, jobs, func(r embedding.Result) {
		mu.Lock()
		results = append(results, r)
		mu.Unlock()
	}, timeout)
	return results, err
}

func TestPool_Run_EmbedsAllJobs(t *testing.T) {
	embedder := &fakeEmbedder{delay: time.Millisecond}
	pool := embedding.NewPool(embedder, 3)

	jobs := make([]embedding.Job, 10)
	for i := range jobs {
		jobs[i] = embedding.Job{ID: int64(i), Text: "hello"}
	}

	results, err := collectResults(pool, context.Background(), jobs, time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != len(jobs) {
		t.Fatalf("expected %d results, got %d", len(jobs), len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("job %d: unexpected error: %v", r.ID, r.Err)
		}
		if len(r.Embedding) != 1 || r.Embedding[0] != float32(len("hello")) {
			t.Errorf("job %d: unexpected embedding %v", r.ID, r.Embedding)
		}
	}
}

func TestPool_Run_BoundsConcurrency(t *testing.T) {
	embedder := &fakeEmbedder{delay: 20 * time.Millisecond}
	const workers = 3
	pool := embedding.NewPool(embedder, workers)

	jobs := make([]embedding.Job, 12)
	for i := range jobs {
		jobs[i] = embedding.Job{ID: int64(i), Text: "x"}
	}

	if _, err := collectResults(pool, context.Background(), jobs, time.Second); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := embedder.maxObservedConcurrency(); got > workers {
		t.Fatalf("observed %d concurrent Embed calls, pool only allows %d", got, workers)
	}
}

func TestPool_Run_ZeroOrNegativeWorkersTreatedAsOne(t *testing.T) {
	embedder := &fakeEmbedder{delay: time.Millisecond}
	pool := embedding.NewPool(embedder, 0)

	results, err := collectResults(pool, context.Background(), []embedding.Job{{ID: 1, Text: "a"}}, time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

// blockingEmbedder ignores context cancellation entirely, to exercise
// Run's shutdown-timeout path deterministically.
type blockingEmbedder struct {
	started chan struct{}
	unblock chan struct{}
}

func (b *blockingEmbedder) Embed(context.Context, string) ([]float32, error) {
	close(b.started)
	<-b.unblock
	return nil, errors.New("blockingEmbedder: should not reach normal completion in this test")
}

func TestPool_Run_TimesOutOnUnresponsiveWorker(t *testing.T) {
	embedder := &blockingEmbedder{started: make(chan struct{}), unblock: make(chan struct{})}
	pool := embedding.NewPool(embedder, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const shutdownTimeout = 50 * time.Millisecond
	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		errCh <- pool.Run(ctx, []embedding.Job{{ID: 1, Text: "x"}}, func(embedding.Result) {}, shutdownTimeout)
	}()

	<-embedder.started
	cancel()

	err := <-errCh
	elapsed := time.Since(start)
	close(embedder.unblock) // let the stuck worker goroutine finish so it doesn't leak into later tests

	if err == nil {
		t.Fatal("expected a shutdown-timeout error")
	}
	if elapsed < shutdownTimeout {
		t.Fatalf("returned before shutdownTimeout elapsed: %s", elapsed)
	}
	if elapsed > 5*shutdownTimeout {
		t.Fatalf("returned too late — timeout doesn't look honored: %s", elapsed)
	}
}

func settledGoroutineCount() int {
	for i := 0; i < 20; i++ {
		runtime.Gosched()
	}
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	return runtime.NumGoroutine()
}

func TestPool_Run_NoGoroutineLeak(t *testing.T) {
	baseline := settledGoroutineCount()

	embedder := &fakeEmbedder{delay: time.Millisecond}
	pool := embedding.NewPool(embedder, 4)

	for i := 0; i < 20; i++ {
		jobs := make([]embedding.Job, 10)
		for j := range jobs {
			jobs[j] = embedding.Job{ID: int64(j), Text: "hello"}
		}
		results, err := collectResults(pool, context.Background(), jobs, time.Second)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if len(results) != len(jobs) {
			t.Fatalf("run %d: expected %d results, got %d", i, len(jobs), len(results))
		}
	}

	after := settledGoroutineCount()
	if after > baseline+2 {
		t.Fatalf("possible goroutine leak: baseline=%d after=%d", baseline, after)
	}
}
