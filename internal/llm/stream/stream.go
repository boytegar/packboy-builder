package stream

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/llm/llmerr"
	"github.com/boytegar/packboy-builder/internal/log"
)

// State tracks common streaming response state across provider implementations.
type State struct {
	ProviderName string
	Start        time.Time
	ChunkCount   int
	// PromptChars is optional: set by the provider (or Infer bridge) so Finish
	// can estimate usage when the provider reports zeros.
	PromptChars int
	Response    llm.CompletionResponse

	contentBuf   strings.Builder
	thinkingBuf  strings.Builder
	lastProgress time.Time
}

// NewState creates a new stream state for a provider.
func NewState(providerName string) *State {
	return &State{
		ProviderName: providerName,
		Start:        time.Now(),
	}
}

// EstimatePromptChars sums the conversational text an input-token estimate
// should count: the system prompt plus each message's content, thinking, tool
// calls and tool results. It intentionally mirrors the coarse char/4 usage
// estimate and is not a tokenizer.
func EstimatePromptChars(sys string, msgs []core.Message) int {
	n := len(sys)
	for _, m := range msgs {
		n += len(m.Content) + len(m.Thinking)
		for _, tc := range m.ToolCalls {
			n += len(tc.Name) + len(tc.Input)
		}
		if m.ToolResult != nil {
			n += len(m.ToolResult.Content)
		}
	}
	return n
}

// Count records one more upstream stream event/chunk.
func (s *State) Count() {
	s.ChunkCount++
}

// progressMinInterval throttles keepalive emits so a high-rate tool-arg stream
// does not flood the Infer bridge with empty chunks.
const progressMinInterval = time.Second

// Progress emits a throttled keepalive chunk so streamInfer rearms its idle
// timer during silent-but-alive phases (tool-arg deltas, usage-only frames).
// Safe to call on every Count(); the throttle drops most of them.
func (s *State) Progress(ctx context.Context, ch chan<- llm.StreamChunk) {
	now := time.Now()
	if !s.lastProgress.IsZero() && now.Sub(s.lastProgress) < progressMinInterval {
		return
	}
	s.lastProgress = now
	send(ctx, ch, llm.StreamChunk{Type: llm.ChunkTypeProgress})
}

// errIncompleteStream is the crush/fantasy NewIncompleteStreamError equivalent:
// the SSE/iterator ended without a terminal signal (message_stop / finish_reason
// / response.completed). Marked retryable; cause is UnexpectedEOF so network
// classification also matches.
var errIncompleteStream = fmt.Errorf("stream closed without terminal signal: %w", io.ErrUnexpectedEOF)

// FinishOrTruncated finishes normally when the provider saw a terminal event
// (or EnsureToolUseStopReason already set a stop). Otherwise fails retryable —
// a silent EOF mid-SSE must not look like a clean end_turn.
func (s *State) FinishOrTruncated(ctx context.Context, ch chan<- llm.StreamChunk, sawTerminal bool) {
	if !sawTerminal && s.Response.StopReason == "" {
		if err := ctx.Err(); err != nil {
			s.Fail(ctx, ch, err)
			return
		}
		s.Fail(ctx, ch, llmerr.MarkRetryable(errIncompleteStream))
		return
	}
	s.Finish(ctx, ch)
}

// send forwards chunk to ch, aborting on ctx cancellation so a goroutine
// holding the stream doesn't wedge forever when the consumer (streamInfer)
// has bailed out via its own ctx.Done branch.
func send(ctx context.Context, ch chan<- llm.StreamChunk, chunk llm.StreamChunk) bool {
	select {
	case ch <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// EmitText forwards a text delta and accumulates it into the response.
func (s *State) EmitText(ctx context.Context, ch chan<- llm.StreamChunk, text string) {
	if text == "" {
		return
	}
	if !send(ctx, ch, llm.StreamChunk{Type: llm.ChunkTypeText, Text: text}) {
		return
	}
	s.contentBuf.WriteString(text)
}

// EmitThinking forwards a thinking delta and accumulates it into the response.
func (s *State) EmitThinking(ctx context.Context, ch chan<- llm.StreamChunk, text string) {
	if text == "" {
		return
	}
	if !send(ctx, ch, llm.StreamChunk{Type: llm.ChunkTypeThinking, Text: text}) {
		return
	}
	s.thinkingBuf.WriteString(text)
}

// UpdateUsage updates the tracked usage values when the provider emits them.
func (s *State) UpdateUsage(inputTokens, outputTokens int) {
	if inputTokens > 0 {
		s.Response.Usage.InputTokens = inputTokens
	}
	if outputTokens > 0 {
		s.Response.Usage.OutputTokens = outputTokens
	}
}

// UpdateCacheUsage records prompt-caching token counts from the provider response.
func (s *State) UpdateCacheUsage(cacheCreation, cacheRead int) {
	if cacheCreation > 0 {
		s.Response.Usage.CacheCreationTokens = cacheCreation
	}
	if cacheRead > 0 {
		s.Response.Usage.CacheReadTokens = cacheRead
	}
}

// AddToolCallsSorted appends tool calls from an indexed accumulator in stable index order.
func (s *State) AddToolCallsSorted(toolCalls map[int]*core.ToolCall) {
	for _, idx := range slices.Sorted(maps.Keys(toolCalls)) {
		s.Response.ToolCalls = append(s.Response.ToolCalls, *toolCalls[idx])
	}
}

// AddToolCallsByKey appends tool calls from a string-keyed accumulator in stable key order.
func (s *State) AddToolCallsByKey(toolCalls map[string]*core.ToolCall) {
	for _, key := range slices.Sorted(maps.Keys(toolCalls)) {
		s.Response.ToolCalls = append(s.Response.ToolCalls, *toolCalls[key])
	}
}

// EnsureToolUseStopReason infers tool_use when tool calls exist but no stop reason was set.
func (s *State) EnsureToolUseStopReason() {
	if len(s.Response.ToolCalls) > 0 && s.Response.StopReason == "" {
		s.Response.StopReason = core.StopToolUse
	}
}

// HasContent reports whether the stream has accumulated any text, thinking,
// or tool calls so far. Used to decide whether to finish gracefully on a
// mid-stream error (keeping partial output) or to fail (no output to keep).
func (s *State) HasContent() bool {
	return s.contentBuf.Len() > 0 || s.thinkingBuf.Len() > 0 || len(s.Response.ToolCalls) > 0
}

// Fail logs and emits a terminal error chunk.
func (s *State) Fail(ctx context.Context, ch chan<- llm.StreamChunk, err error) {
	log.LogError(s.ProviderName, err)
	send(ctx, ch, llm.StreamChunk{
		Type:  llm.ChunkTypeError,
		Error: err,
	})
}

// Finish logs stream completion, logs the final response, and emits the done chunk.
// It copies the response so the receiver does not retain a pointer into State,
// allowing the State (and its string builders) to be GC'd.
//
// The Done chunk is sent via the ctx-aware send helper so a cancel that races
// the provider's natural stream completion doesn't wedge this goroutine on an
// unbuffered channel after the bridge has already exited via ctx.Done.
func (s *State) Finish(ctx context.Context, ch chan<- llm.StreamChunk) {
	s.Response.Content = s.contentBuf.String()
	s.Response.Thinking = s.thinkingBuf.String()
	s.estimateUsageIfMissing()
	log.LogStreamDone(s.ProviderName, time.Since(s.Start), s.ChunkCount)
	log.LogResponseCtx(ctx, s.ProviderName, s.Response)
	resp := s.Response // shallow copy — breaks the pointer into State
	send(ctx, ch, llm.StreamChunk{
		Type:     llm.ChunkTypeDone,
		Response: &resp,
	})
}

// estimateUsageIfMissing fills zero In/Out fields with a crush-style char/4
// estimate so compaction and the status bar still see a prompt size when the
// provider omits usage (common on early disconnect + partial finish paths).
//
// Per-field: providers that emit only OutputTokens (Anthropic message_delta,
// openaicompat salvage) must not suppress the InputTokens estimate. Cache and
// reasoning counts from the provider are left untouched.
func (s *State) estimateUsageIfMissing() {
	u := &s.Response.Usage
	outChars := s.contentBuf.Len() + s.thinkingBuf.Len()
	for _, tc := range s.Response.ToolCalls {
		outChars += len(tc.ID) + len(tc.Name) + len(tc.Input)
	}
	if u.InputTokens == 0 && s.PromptChars > 0 {
		u.InputTokens = (s.PromptChars + 3) / 4
	}
	if u.OutputTokens == 0 && outChars > 0 {
		u.OutputTokens = (outChars + 3) / 4
	}
	// Only synthesize Total when neither the provider nor prior path set it;
	// keep provider TotalTokens if present.
	if u.TotalTokens == 0 && (u.InputTokens != 0 || u.OutputTokens != 0) {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
}
