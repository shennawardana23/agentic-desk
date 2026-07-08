package feedback_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/agentloop/feedback"
)

type capturingWriter struct {
	decision      feedback.Decision
	note          string
	memoryEntryID *int64
	err           error
	calls         int
}

func (w *capturingWriter) WriteSignal(_ context.Context, decision feedback.Decision, note string, memoryEntryID *int64) error {
	w.calls++
	w.decision = decision
	w.note = note
	w.memoryEntryID = memoryEntryID
	return w.err
}

func TestRecorder_Record_WritesThroughToWriter(t *testing.T) {
	writer := &capturingWriter{}
	recorder := &feedback.Recorder{Writer: writer}
	id := int64(42)

	if err := recorder.Record(context.Background(), feedback.DecisionCorrect, "fixed the typo", &id); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if writer.calls != 1 {
		t.Fatalf("expected WriteSignal called exactly once, got %d", writer.calls)
	}
	if writer.decision != feedback.DecisionCorrect {
		t.Errorf("decision = %q, want %q", writer.decision, feedback.DecisionCorrect)
	}
	if writer.note != "fixed the typo" {
		t.Errorf("note = %q, want %q", writer.note, "fixed the typo")
	}
	if writer.memoryEntryID == nil || *writer.memoryEntryID != 42 {
		t.Errorf("memoryEntryID = %v, want pointer to 42", writer.memoryEntryID)
	}
}

func TestRecorder_Record_NilMemoryEntryID(t *testing.T) {
	writer := &capturingWriter{}
	recorder := &feedback.Recorder{Writer: writer}

	if err := recorder.Record(context.Background(), feedback.DecisionApprove, "", nil); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if writer.memoryEntryID != nil {
		t.Errorf("expected nil memoryEntryID, got %v", writer.memoryEntryID)
	}
}

func TestRecorder_Record_PropagatesWriterError(t *testing.T) {
	writer := &capturingWriter{err: errors.New("db down")}
	recorder := &feedback.Recorder{Writer: writer}

	if err := recorder.Record(context.Background(), feedback.DecisionReject, "bad output", nil); err == nil {
		t.Fatal("expected the writer's error to propagate")
	}
}
