package core

import (
	"context"
	"testing"
	"time"
)

// The turn loop reports Content from a single tracked value rather than from
// each return site. These tests pin the two exits that used to supply it by
// hand and got it wrong: the step cap substituted a status string, and every
// cancellation path substituted "". Both discarded work the model had already
// done and the caller had already paid for.

// noopTool lets a scripted response carry a tool call, which is what keeps the
// turn loop iterating instead of ending on the first step.
type noopTool struct{}

func (noopTool) Name() string        { return "noop" }
func (noopTool) Description() string { return "does nothing" }
func (noopTool) Schema() ToolSchema  { return ToolSchema{Name: "noop"} }
func (noopTool) Execute(context.Context, map[string]any) (string, error) {
	return "done", nil
}

// talkingLLM answers every inference with text plus a tool call, so the turn
// never reaches end_turn on its own and the loop exits through a guard.
type talkingLLM struct {
	text string
	// quietAfterFirst silences every step past the first, producing a turn that
	// ends on a tool-only step carrying no assistant text.
	quietAfterFirst bool
	calls           int
	onInfer         func(call int)
}

func (t *talkingLLM) InputLimit() int { return 0 }

func (t *talkingLLM) Infer(_ context.Context, _ InferRequest) (<-chan Chunk, error) {
	t.calls++
	if t.onInfer != nil {
		t.onInfer(t.calls)
	}
	text := t.text
	if t.quietAfterFirst && t.calls > 1 {
		text = ""
	}
	ch := make(chan Chunk, 1)
	go func() {
		defer close(ch)
		ch <- Chunk{Done: true, Response: &InferResponse{
			Content:    text,
			StopReason: StopToolUse,
			ToolCalls:  []ToolCall{{ID: "call-1", Name: "noop", Input: "{}"}},
		}}
	}()
	return ch, nil
}

// newContentAgent mirrors newRetryAgent in retry_test.go: build, seed a user
// message, and drain the outbox, which emit() blocks on once the buffer fills.
func newContentAgent(llm LLM, maxSteps int) *agent {
	ag := NewAgent(Config{
		ID:       "test",
		LLM:      llm,
		System:   NewSystem(),
		Tools:    NewTools(noopTool{}),
		MaxSteps: maxSteps,
	}).(*agent)
	ag.append(Message{Role: RoleUser, Content: "go"})
	go func() {
		for range ag.Outbox() {
		}
	}()
	return ag
}

func TestThinkActReportsModelOutputWhenStepCapHits(t *testing.T) {
	llm := &talkingLLM{text: "partial analysis of the repo"}
	ag := newContentAgent(llm, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := ag.ThinkAct(ctx)
	if err != nil {
		t.Fatalf("ThinkAct returned error: %v", err)
	}
	if result.StopReason != StopMaxSteps {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopMaxSteps)
	}
	// The cap is already reported through StopReason; Content must not be
	// overwritten with a status string, or a subagent that hits its cap hands
	// its parent that string in place of the work.
	if result.Content != llm.text {
		t.Errorf("Content = %q, want the model's last output %q", result.Content, llm.text)
	}
	if result.Steps != 2 {
		t.Errorf("Steps = %d, want 2", result.Steps)
	}
}

func TestThinkActPreservesPartialOutputOnCancel(t *testing.T) {
	// Bounded as well as cancellable: talkingLLM always answers with a tool
	// call, so if the cancel hook below ever stopped firing the loop would spin
	// until go test's global timeout instead of failing here.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	llm := &talkingLLM{text: "found the bug in fs_store"}
	// Cancel on the second inference, so step one has fully completed and its
	// assistant message is already appended. Cancelling during the first
	// inference would be a different case: no assistant message exists yet, and
	// an empty Content is then the honest answer rather than a lost one.
	llm.onInfer = func(call int) {
		if call == 2 {
			cancel()
		}
	}

	ag := newContentAgent(llm, 0)

	result, err := ag.ThinkAct(ctx)
	if err == nil {
		t.Fatal("ThinkAct returned nil error on a cancelled turn")
	}
	if result == nil {
		t.Fatal("ThinkAct returned nil Result on cancel; observers need the turn boundary")
	}
	if result.StopReason != StopCancelled {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopCancelled)
	}
	// subagent.buildUnfinishedAgentResult documents that a cancelled run's
	// partial content travels back with the error, and RunBackground gates that
	// on Content != "". An empty string here makes that feature dead code.
	if result.Content != llm.text {
		t.Errorf("Content = %q, want the partial output %q", result.Content, llm.text)
	}
}

// TestThinkActReportsEmptyContentWhenTurnEndsOnToolOnlyStep pins the deliberate
// half of the tracking rule. lastContent follows the newest assistant message
// unconditionally, so a turn whose last step is pure tool calls reports "" —
// it does not fall back to narration from an earlier step. That fallback would
// present stale text as this turn's outcome, so the emptiness is the intended
// answer rather than a gap to close.
func TestThinkActReportsEmptyContentWhenTurnEndsOnToolOnlyStep(t *testing.T) {
	llm := &talkingLLM{text: "starting the sweep", quietAfterFirst: true}
	ag := newContentAgent(llm, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := ag.ThinkAct(ctx)
	if err != nil {
		t.Fatalf("ThinkAct returned error: %v", err)
	}
	if result.StopReason != StopMaxSteps {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopMaxSteps)
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want \"\" — the last step carried no assistant text", result.Content)
	}
}

// TestSetMaxStepsAdjustsCapMidTurn pins that SetMaxSteps takes effect on the
// next iteration without interrupting the in-flight turn. A cap raised between
// steps must let the loop continue past the old limit; a cap lowered to 0 must
// lift the limit entirely (0 = unlimited).
func TestSetMaxStepsAdjustsCapMidTurn(t *testing.T) {
	llm := &talkingLLM{text: "working"}
	// Start with a cap of 2; the onInfer hook raises it to 5 after step 1.
	ag := newContentAgent(llm, 2)
	llm.onInfer = func(call int) {
		if call == 1 {
			ag.SetMaxSteps(5)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := ag.ThinkAct(ctx)
	if err != nil {
		t.Fatalf("ThinkAct returned error: %v", err)
	}
	if result.Steps < 3 {
		t.Errorf("Steps = %d, want ≥3 — the raised cap should have let the loop run past the old 2", result.Steps)
	}
}

// TestSetMaxStepsToZeroLiftsCap pins that setting 0 lifts the cap entirely.
// The loop must run until the model ends the turn, not stop on the old limit.
func TestSetMaxStepsToZeroLiftsCap(t *testing.T) {
	// quietAfterFirst makes the LLM emit a tool call every step, so the loop
	// never self-terminates; with the cap lifted it would spin forever. Cap
	// it again after a few steps via the hook to bound the test.
	llm := &talkingLLM{text: "working"}
	ag := newContentAgent(llm, 2)
	llm.onInfer = func(call int) {
		if call == 1 {
			ag.SetMaxSteps(0) // lift the cap
		}
		if call == 4 {
			ag.SetMaxSteps(4) // re-impose to bound the test
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := ag.ThinkAct(ctx)
	if err != nil {
		t.Fatalf("ThinkAct returned error: %v", err)
	}
	if result.Steps != 4 {
		t.Errorf("Steps = %d, want 4 — cap lifted on step 1, re-imposed at 4", result.Steps)
	}
	if result.StopReason != StopMaxSteps {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopMaxSteps)
	}
}

// TestMaxStepsGetterReflectsSet pins the read-side API.
func TestMaxStepsGetterReflectsSet(t *testing.T) {
	ag := newContentAgent(&talkingLLM{text: "x"}, 0)
	if got := ag.MaxSteps(); got != 0 {
		t.Fatalf("MaxSteps() = %d, want 0 (unlimited) at construction", got)
	}
	ag.SetMaxSteps(42)
	if got := ag.MaxSteps(); got != 42 {
		t.Fatalf("MaxSteps() = %d, want 42 after SetMaxSteps", got)
	}
	ag.SetMaxSteps(0)
	if got := ag.MaxSteps(); got != 0 {
		t.Fatalf("MaxSteps() = %d, want 0 after lifting", got)
	}
}
