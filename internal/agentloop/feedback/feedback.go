// Package feedback captures a human's decision at an Agent Loop
// escalation point (or an explicit thumbs-up/down on a committed
// result) and writes it as a preference signal. It depends only on its
// own narrow SignalWriter interface, never on internal/secondbrain
// directly — secondbrain.Store satisfies it, but through an adapter
// defined wherever the app is wired together, not here. This keeps the
// Agent Loop primitive reusable outside this app (design doc Section 5).
package feedback

import "context"

// Decision is a human's response at an escalation point or an
// explicit thumbs-up/down on a committed result.
type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionCorrect Decision = "correct"
	DecisionReject  Decision = "reject"
)

// SignalWriter persists one recorded Decision. memoryEntryID is nil
// when the decision isn't tied to a specific memory entry.
type SignalWriter interface {
	WriteSignal(ctx context.Context, decision Decision, note string, memoryEntryID *int64) error
}

// Recorder captures a human Decision and writes it through a
// SignalWriter.
type Recorder struct {
	Writer SignalWriter
}

// Record writes decision through the Recorder's SignalWriter.
func (r *Recorder) Record(ctx context.Context, decision Decision, note string, memoryEntryID *int64) error {
	return r.Writer.WriteSignal(ctx, decision, note, memoryEntryID)
}
