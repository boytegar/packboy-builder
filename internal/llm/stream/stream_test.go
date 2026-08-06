package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/llm"
)

func TestStateEmitsAndAccumulatesChunks(t *testing.T) {
	ch := make(chan llm.StreamChunk, 8)
	state := NewState("test")

	state.EmitText(context.Background(), ch, "hello")
	state.EmitThinking(context.Background(), ch, "thinking")

	// Verify streaming chunks emitted in order. Tool-call deltas are not
	// streamed as chunks — completed calls ride in the final Response.
	msgs := []llm.StreamChunk{<-ch, <-ch}
	if msgs[0].Type != llm.ChunkTypeText || msgs[0].Text != "hello" {
		t.Fatalf("unexpected text chunk: %#v", msgs[0])
	}
	if msgs[1].Type != llm.ChunkTypeThinking || msgs[1].Text != "thinking" {
		t.Fatalf("unexpected thinking chunk: %#v", msgs[1])
	}

	// Content and thinking accumulate in internal buffers and flush on Finish.
	state.Finish(context.Background(), ch)
	doneChunk := <-ch
	if got := doneChunk.Response.Content; got != "hello" {
		t.Fatalf("expected content to accumulate, got %q", got)
	}
	if got := doneChunk.Response.Thinking; got != "thinking" {
		t.Fatalf("expected thinking to accumulate, got %q", got)
	}
}

func TestStateAddsToolCallsInStableOrder(t *testing.T) {
	byIndex := NewState("test")
	byIndex.AddToolCallsSorted(map[int]*core.ToolCall{
		2: {ID: "c", Name: "third"},
		0: {ID: "a", Name: "first"},
		1: {ID: "b", Name: "second"},
	})

	if len(byIndex.Response.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(byIndex.Response.ToolCalls))
	}
	if byIndex.Response.ToolCalls[0].ID != "a" || byIndex.Response.ToolCalls[1].ID != "b" || byIndex.Response.ToolCalls[2].ID != "c" {
		t.Fatalf("tool calls were not sorted by index: %#v", byIndex.Response.ToolCalls)
	}

	byKey := NewState("test")
	byKey.AddToolCallsByKey(map[string]*core.ToolCall{
		"z": {ID: "3", Name: "third"},
		"a": {ID: "1", Name: "first"},
		"m": {ID: "2", Name: "second"},
	})

	if len(byKey.Response.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(byKey.Response.ToolCalls))
	}
	if byKey.Response.ToolCalls[0].ID != "1" || byKey.Response.ToolCalls[1].ID != "2" || byKey.Response.ToolCalls[2].ID != "3" {
		t.Fatalf("tool calls were not sorted by key: %#v", byKey.Response.ToolCalls)
	}
}

func TestStateEnsureToolUseStopReason(t *testing.T) {
	state := NewState("test")
	state.Response.ToolCalls = []core.ToolCall{{ID: "tool-1", Name: "Read"}}
	state.EnsureToolUseStopReason()

	if got := state.Response.StopReason; got != "tool_use" {
		t.Fatalf("expected tool_use stop reason, got %q", got)
	}

	state.Response.StopReason = "max_tokens"
	state.EnsureToolUseStopReason()
	if got := state.Response.StopReason; got != "max_tokens" {
		t.Fatalf("expected existing stop reason to be preserved, got %q", got)
	}
}

func TestStateFailAndFinishEmitTerminalChunks(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")

	// Accumulate content via EmitText (content flushes in Finish)
	state.EmitText(context.Background(), ch, "done")
	<-ch // drain the text chunk

	state.Fail(context.Background(), ch, errors.New("boom"))
	errChunk := <-ch
	if errChunk.Type != llm.ChunkTypeError || errChunk.Error == nil || errChunk.Error.Error() != "boom" {
		t.Fatalf("unexpected error chunk: %#v", errChunk)
	}

	state.Finish(context.Background(), ch)
	doneChunk := <-ch
	if doneChunk.Type != llm.ChunkTypeDone || doneChunk.Response == nil {
		t.Fatalf("unexpected done chunk: %#v", doneChunk)
	}
	if doneChunk.Response.Content != "done" {
		t.Fatalf("expected final response content to be preserved, got %#v", doneChunk.Response)
	}
}

func TestFinishOrTruncatedFailsWhenNoTerminalSignal(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")
	// No content, no stop reason, no tool call: a terminal signal is required.
	state.FinishOrTruncated(context.Background(), ch, false)
	got := <-ch
	if got.Type != llm.ChunkTypeError || got.Error == nil {
		t.Fatalf("expected a retryable error chunk, got %#v", got)
	}
	var rt core.RetryableError
	if !errors.As(got.Error, &rt) {
		t.Fatalf("truncation error %v is not retryable", got.Error)
	}
}

func TestFinishOrTruncatedFinishesWhenTerminalSeen(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")
	state.EmitText(context.Background(), ch, "hello")
	<-ch
	// Not passing sawTerminal, but a stop reason is set → treat as finished.
	state.Response.StopReason = core.StopEndTurn
	state.FinishOrTruncated(context.Background(), ch, false)
	got := <-ch
	if got.Type != llm.ChunkTypeDone || got.Response == nil {
		t.Fatalf("expected a done chunk, got %#v", got)
	}
}

func TestEstimatePromptCharsFillsZeroUsage(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")
	state.PromptChars = 1000                                               // 250 tokens by the char/4 estimate
	state.EmitText(context.Background(), ch, "abcdefgh ijklmnop qrstuvwx") // 26 chars → 7 out
	<-ch
	state.Finish(context.Background(), ch)
	done := <-ch
	if done.Response == nil {
		t.Fatal("missing done chunk")
	}
	if done.Response.Usage.InputTokens != 250 {
		t.Fatalf("InputTokens = %d, want 250", done.Response.Usage.InputTokens)
	}
	if done.Response.Usage.OutputTokens != 7 {
		t.Fatalf("OutputTokens = %d, want 7", done.Response.Usage.OutputTokens)
	}
	if done.Response.Usage.TotalTokens != 257 {
		t.Fatalf("TotalTokens = %d, want 257", done.Response.Usage.TotalTokens)
	}
}

func TestEstimateUsagePreservesProvidedUsage(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")
	state.PromptChars = 1000
	state.UpdateUsage(11, 7)
	state.Finish(context.Background(), ch)
	done := <-ch
	if got := done.Response.Usage.InputTokens; got != 11 {
		t.Fatalf("InputTokens = %d, want provider-provided 11", got)
	}
	if got := done.Response.Usage.OutputTokens; got != 7 {
		t.Fatalf("OutputTokens = %d, want provider-provided 7", got)
	}
}

// Providers that only emit OutputTokens (Anthropic message_delta, salvage paths)
// must still get an InputTokens estimate from PromptChars — all-or-nothing bail
// left status-bar/compaction at in:0.
func TestEstimateUsageFillsMissingInputOnly(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")
	state.PromptChars = 1000 // → 250
	state.UpdateUsage(0, 42) // provider output only
	state.Finish(context.Background(), ch)
	done := <-ch
	if got := done.Response.Usage.InputTokens; got != 250 {
		t.Fatalf("InputTokens = %d, want estimated 250", got)
	}
	if got := done.Response.Usage.OutputTokens; got != 42 {
		t.Fatalf("OutputTokens = %d, want provider 42", got)
	}
	if got := done.Response.Usage.TotalTokens; got != 292 {
		t.Fatalf("TotalTokens = %d, want 292", got)
	}
}

func TestEstimateUsagePreservesCacheWhenFillingInput(t *testing.T) {
	ch := make(chan llm.StreamChunk, 4)
	state := NewState("test")
	state.PromptChars = 400 // → 100
	state.UpdateUsage(0, 5)
	state.UpdateCacheUsage(10, 90)
	state.Finish(context.Background(), ch)
	done := <-ch
	u := done.Response.Usage
	if u.InputTokens != 100 {
		t.Fatalf("InputTokens = %d, want 100", u.InputTokens)
	}
	if u.CacheCreationTokens != 10 || u.CacheReadTokens != 90 {
		t.Fatalf("cache tokens clobbered: creation=%d read=%d", u.CacheCreationTokens, u.CacheReadTokens)
	}
}

func TestEstimatePromptCharsCountsContent(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleUser, Content: "hello world"},
		{Role: core.RoleAssistant, Content: "abc", ToolCalls: []core.ToolCall{{Name: "Read", Input: "{\"x\":1}"}}},
		{Role: core.RoleUser, ToolResult: &core.ToolResult{Content: "big result"}},
	}
	got := EstimatePromptChars("sys", msgs)
	// 3 (sys) + 11 (hello world) + 3 (abc) + 4+7 (Read + {"x":1}) + 10 (big result)
	want := 3 + 11 + 3 + 4 + 7 + 10
	if got != want {
		t.Fatalf("EstimatePromptChars = %d, want %d", got, want)
	}
}
