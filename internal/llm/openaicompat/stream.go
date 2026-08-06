package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/openai/openai-go/v3"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/llm"
	streamutil "github.com/boytegar/packboy-builder/internal/llm/stream"
	"github.com/boytegar/packboy-builder/internal/log"
)

// ChatStreamConfig contains provider-specific knobs for OpenAI-compatible
// Chat Completions streaming.
type ChatStreamConfig struct {
	Client           openai.Client
	ProviderName     string
	Options          llm.CompletionOptions
	ConvertAssistant func(core.Message) openai.ChatCompletionMessageParamUnion
	ConfigureParams  func(*openai.ChatCompletionNewParams)
	ExtractReasoning bool
}

// StreamChatCompletions streams an OpenAI-compatible Chat Completions request.
func StreamChatCompletions(ctx context.Context, cfg ChatStreamConfig) <-chan llm.StreamChunk {
	ch := make(chan llm.StreamChunk)

	go func() {
		defer close(ch)

		opts := cfg.Options
		messages := ConvertMessages(opts.Messages, opts.SystemPrompt, cfg.ConvertAssistant)

		params := openai.ChatCompletionNewParams{
			Model:    opts.Model,
			Messages: messages,
			StreamOptions: openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: openai.Bool(true),
			},
		}
		if opts.MaxTokens > 0 {
			params.MaxCompletionTokens = openai.Int(int64(opts.MaxTokens))
		}
		if opts.Temperature > 0 {
			params.Temperature = openai.Float(opts.Temperature)
		}
		if len(opts.Tools) > 0 {
			params.Tools = ConvertTools(opts.Tools)
		}
		if cfg.ConfigureParams != nil {
			cfg.ConfigureParams(&params)
		}

		log.LogRequestCtx(ctx, cfg.ProviderName, opts.Model, opts)

		stream := cfg.Client.Chat.Completions.NewStreaming(ctx, params)
		defer stream.Close() // release the HTTP body on every exit

		state := streamutil.NewState(cfg.ProviderName)
		state.PromptChars = streamutil.EstimatePromptChars(opts.SystemPrompt, opts.Messages)
		toolCalls := make(map[int]*core.ToolCall)
		// sawFinish tracks whether any choice delivered a finish_reason. A socket
		// close before one is a truncated stream and must be retried (unless the
		// JSON-syntax salvage path below preserves partial content).
		sawFinish := false

		for stream.Next() {
			chunk := stream.Current()
			state.Count()
			state.Progress(ctx, ch)

			for _, choice := range chunk.Choices {
				if cfg.ExtractReasoning {
					if content := ExtractReasoningContent(choice.Delta.RawJSON()); content != "" {
						state.EmitThinking(ctx, ch, content)
					}
				}
				if choice.Delta.Content != "" {
					state.EmitText(ctx, ch, choice.Delta.Content)
				}

				for _, tc := range choice.Delta.ToolCalls {
					idx := int(tc.Index)
					if _, exists := toolCalls[idx]; !exists {
						toolCalls[idx] = &core.ToolCall{ID: tc.ID, Name: tc.Function.Name}
					}
					if tc.Function.Arguments != "" {
						toolCalls[idx].Input += tc.Function.Arguments
					}
				}

				if choice.FinishReason != "" {
					state.Response.StopReason = MapFinishReason(choice.FinishReason)
					sawFinish = true
				}
			}

			// prompt_tokens is the full prompt; the cached slice lives under
			// prompt_tokens_details. Split into the Anthropic fresh/cache-read
			// convention the app assumes — see SplitInputTokens.
			fresh, cached := SplitInputTokens(int(chunk.Usage.PromptTokens), int(chunk.Usage.PromptTokensDetails.CachedTokens))
			state.UpdateUsage(fresh, int(chunk.Usage.CompletionTokens))
			state.UpdateCacheUsage(0, cached)
		}

		if err := stream.Err(); err != nil {
			// Follows crush's SSE resilience pattern (client/proto.go): a
			// malformed/truncated SSE line should not kill a stream that has
			// already delivered partial content. The openai-go SDK's SSE
			// parser stores a json.SyntaxError ("unexpected end of JSON
			// input") in stream.Err() when a data: line is truncated. When
			// that happens mid-stream, finish with the partial content the
			// user already received instead of discarding it.
			if isJSONSyntaxError(err) && state.HasContent() {
				log.LogError(cfg.ProviderName, err)
				state.AddToolCallsSorted(toolCalls)
				state.EnsureToolUseStopReason()
				state.Finish(ctx, ch)
				return
			}
			state.Fail(ctx, ch, NormalizeAPIError(cfg.ProviderName, err))
			return
		}

		state.AddToolCallsSorted(toolCalls)
		state.EnsureToolUseStopReason()
		state.FinishOrTruncated(ctx, ch, sawFinish)
	}()

	return ch
}

// isJSONSyntaxError reports whether err is a JSON parsing failure such as
// "unexpected end of JSON input". The openai-go SDK's SSE reader stores this
// in stream.Err() when a data: line is truncated. We detect it via
// errors.As against *json.SyntaxError and also by message substring (the
// SDK may wrap the error in its own type that doesn't satisfy errors.As).
func isJSONSyntaxError(err error) bool {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	return err != nil && containsErrorMsg(err, "unexpected end of JSON input")
}

// containsErrorMsg checks whether err's message contains substr, walking
// the error chain via errors.Unwrap.
func containsErrorMsg(err error, substr string) bool {
	for {
		if err == nil {
			return false
		}
		if strings.Contains(err.Error(), substr) {
			return true
		}
		err = errors.Unwrap(err)
	}
}
