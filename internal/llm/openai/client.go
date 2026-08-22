package openai

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/llm/llmerr"
	"github.com/boytegar/packboy-builder/internal/llm/openaicompat"
	streamutil "github.com/boytegar/packboy-builder/internal/llm/stream"
	"github.com/boytegar/packboy-builder/internal/log"
)

// Client implements the Provider interface using the OpenAI SDK
type Client struct {
	client openai.Client
	name   string
	// subscription selects the ChatGPT Codex backend behavior: requests must be
	// stateless (store=false) and carry encrypted reasoning so it round-trips,
	// and the model list is a static catalog rather than an API call.
	subscription bool
	limitMu      sync.Mutex
	limitCache   map[string]modelTokenLimits
}

// NewClient creates a new OpenAI client with the given SDK client
func NewClient(client openai.Client, name string) *Client {
	return &Client{
		client:     client,
		name:       name,
		limitCache: make(map[string]modelTokenLimits),
	}
}

// Name returns the provider name
func (c *Client) Name() string {
	return c.name
}

// Stream sends a completion request and returns a channel of streaming chunks.
// OpenAI is implemented via the Responses API only.
func (c *Client) Stream(ctx context.Context, opts llm.CompletionOptions) <-chan llm.StreamChunk {
	return c.streamResponses(ctx, opts)
}

// streamResponses implements streaming via the Responses API.
func (c *Client) streamResponses(ctx context.Context, opts llm.CompletionOptions) <-chan llm.StreamChunk {
	ch := make(chan llm.StreamChunk)

	go func() {
		defer close(ch)

		// Convert messages to Responses API input items
		var inputItems responses.ResponseInputParam = make([]responses.ResponseInputItemUnionParam, 0, len(opts.Messages)+1)

		for _, msg := range openaicompat.DropEmptyMessages(llm.SanitizeToolMessages(opts.Messages)) {
			if msg.ToolResult != nil {
				inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
					OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
						CallID: msg.ToolResult.ToolCallID,
						Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
							OfString: openai.Opt(msg.ToolResult.Content),
						},
					},
				})
				continue
			}
			switch msg.Role {
			case core.RoleUser:
				inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
					OfMessage: responseMessageParam(responses.EasyInputMessageRoleUser, msg),
				})
			case core.RoleAssistant:
				// Echo back any reasoning items first: the stateless ChatGPT
				// backend requires a reasoning model's function_call to be
				// preceded by its reasoning item (carried via encrypted_content).
				for _, r := range msg.Reasoning {
					if r.EncryptedContent == "" {
						continue
					}
					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfReasoning: reasoningInputParam(r),
					})
				}
				if len(msg.ToolCalls) > 0 {
					// Add text content as a message if present
					if messageHasResponseContent(msg) {
						inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
							OfMessage: responseMessageParam(responses.EasyInputMessageRoleAssistant, msg),
						})
					}
					// Add each tool call as a separate function_call input item
					for _, tc := range msg.ToolCalls {
						inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
							OfFunctionCall: &responses.ResponseFunctionToolCallParam{
								CallID:    tc.ID,
								Name:      tc.Name,
								Arguments: tc.Input,
							},
						})
					}
				} else {
					if !messageHasResponseContent(msg) {
						continue
					}
					inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
						OfMessage: responseMessageParam(responses.EasyInputMessageRoleAssistant, msg),
					})
				}
			default: // system messages
				if !messageHasResponseContent(msg) {
					continue
				}
				inputItems = append(inputItems, responses.ResponseInputItemUnionParam{
					OfMessage: responseMessageParam(responses.EasyInputMessageRoleSystem, msg),
				})
			}
		}

		// Build request params
		params := responses.ResponseNewParams{
			Model: opts.Model,
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: inputItems,
			},
		}

		if opts.SystemPrompt != "" {
			params.Instructions = openai.Opt(opts.SystemPrompt)
		}

		if opts.MaxTokens > 0 && !c.subscription {
			params.MaxOutputTokens = openai.Opt(int64(opts.MaxTokens))
		}

		// The ChatGPT Codex backend is stateless: it rejects store=true and
		// needs reasoning returned as encrypted content so it can be replayed on
		// the next turn.
		if c.subscription {
			params.Store = openai.Bool(false)
			params.Include = []responses.ResponseIncludable{
				responses.ResponseIncludableReasoningEncryptedContent,
			}
		}

		if opts.Temperature > 0 {
			params.Temperature = openai.Opt(opts.Temperature)
		}

		if opts.ThinkingEffort != "" {
			params.Reasoning = shared.ReasoningParam{
				Effort:  shared.ReasoningEffort(opts.ThinkingEffort),
				Summary: shared.ReasoningSummaryAuto,
			}
		}

		// Add tools if provided
		if len(opts.Tools) > 0 {
			tools := make([]responses.ToolUnionParam, len(opts.Tools))
			for i, t := range opts.Tools {
				var funcParams map[string]any
				if props, ok := t.Parameters.(map[string]any); ok {
					funcParams = props
				}
				tools[i] = responses.ToolUnionParam{
					OfFunction: &responses.FunctionToolParam{
						Name:        t.Name,
						Description: openai.Opt(t.Description),
						Parameters:  funcParams,
					},
				}
			}
			params.Tools = tools
		}

		// Log request
		log.LogRequestCtx(ctx, c.name, opts.Model, opts)

		// Create streaming request
		stream := c.client.Responses.NewStreaming(ctx, params)
		defer stream.Close() // release the HTTP body on every exit

		state := streamutil.NewState(c.name)
		state.PromptChars = streamutil.EstimatePromptChars(opts.SystemPrompt, opts.Messages)

		// Track tool calls by item ID
		toolCalls := make(map[string]*core.ToolCall)
		hasToolCalls := false
		// sawTerminal is set once the Responses API delivered its terminal event
		// (response.completed / response.incomplete). A socket close before one
		// arrives is a truncated stream and must be retried, not accepted as a
		// clean end_turn.
		sawTerminal := false

		// Read stream events
		for stream.Next() {
			event := stream.Current()
			state.Count()
			state.Progress(ctx, ch)

			switch event.Type {
			case "response.output_text.delta":
				delta := event.AsResponseOutputTextDelta()
				state.EmitText(ctx, ch, delta.Delta)

			case "response.reasoning_summary_part.added":
				// The reasoning summary streams as discrete parts, each a self-
				// contained "**headline**\n\nbody" section with no separator
				// between them. Without a break the parts collide — two adjacent
				// bold headlines render as "…truncation****Updating…". Insert a
				// blank line before every part after the first.
				part := event.AsResponseReasoningSummaryPartAdded()
				if part.SummaryIndex > 0 {
					state.EmitThinking(ctx, ch, "\n\n")
				}

			case "response.reasoning_summary_text.delta":
				delta := event.AsResponseReasoningSummaryTextDelta()
				state.EmitThinking(ctx, ch, delta.Delta)

			case "response.reasoning_text.delta":
				delta := event.AsResponseReasoningTextDelta()
				state.EmitThinking(ctx, ch, delta.Delta)

			case "response.output_item.added":
				itemEvent := event.AsResponseOutputItemAdded()
				if itemEvent.Item.Type == "function_call" {
					funcCall := itemEvent.Item.AsFunctionCall()
					hasToolCalls = true
					toolCalls[funcCall.ID] = &core.ToolCall{
						ID:   funcCall.CallID,
						Name: funcCall.Name,
					}
				}

			case "response.function_call_arguments.delta":
				delta := event.AsResponseFunctionCallArgumentsDelta()
				if tc, ok := toolCalls[delta.ItemID]; ok {
					tc.Input += delta.Delta
				}

			case "response.completed":
				completed := event.AsResponseCompleted()
				resp := completed.Response
				sawTerminal = true

				// Capture reasoning items so they can be echoed back on the next
				// stateless (store=false) turn; only the subscription backend
				// includes encrypted_content, so this is empty for the direct API.
				if c.subscription {
					state.Response.Reasoning = extractReasoning(resp.Output)
				}

				// input_tokens is the full prompt; the cached slice lives under
				// input_tokens_details. Split into the Anthropic fresh/cache-read
				// convention the app assumes — see openaicompat.SplitInputTokens.
				fresh, cached := openaicompat.SplitInputTokens(int(resp.Usage.InputTokens), int(resp.Usage.InputTokensDetails.CachedTokens))
				state.UpdateUsage(fresh, int(resp.Usage.OutputTokens))
				state.UpdateCacheUsage(0, cached)

				// Determine stop reason
				switch resp.Status {
				case responses.ResponseStatusCompleted:
					if hasToolCalls {
						state.Response.StopReason = core.StopToolUse
					} else {
						state.Response.StopReason = core.StopEndTurn
					}
				case responses.ResponseStatusIncomplete:
					state.Response.StopReason = core.StopMaxTokens
				case responses.ResponseStatusFailed:
					errMsg := "response failed"
					if resp.Error.Message != "" {
						errMsg = resp.Error.Message
					}
					err := fmt.Errorf("responses API failed: %s", errMsg)
					if retryableResponseError(resp.Error.Code) {
						err = llmerr.MarkRetryable(err)
					} else {
						err = llmerr.MarkNonRetryable(err)
					}
					state.Fail(ctx, ch, err)
					return
				default:
					state.Response.StopReason = core.StopReason(resp.Status)
				}

			case "response.incomplete":
				// A max_tokens / interrupted response is still a terminal event:
				// the stream is complete even though the response ended early.
				sawTerminal = true
				state.Response.StopReason = core.StopMaxTokens
				incomplete := event.AsResponseIncomplete()
				if u := incomplete.Response.Usage; u.InputTokens != 0 || u.OutputTokens != 0 {
					// Same split as response.completed — raw InputTokens is the full
					// prompt; CachedTokens must move to CacheRead or TotalInputTokens
					// double-counts and the status bar / cost skew.
					fresh, cached := openaicompat.SplitInputTokens(int(u.InputTokens), int(u.InputTokensDetails.CachedTokens))
					state.UpdateUsage(fresh, int(u.OutputTokens))
					state.UpdateCacheUsage(0, cached)
				}

			case "error":
				errEvent := event.AsError()
				err := fmt.Errorf("responses API error: %s", errEvent.Message)
				if retryableResponseErrorCode(errEvent.Code) {
					err = llmerr.MarkRetryable(err)
				} else {
					err = llmerr.MarkNonRetryable(err)
				}
				state.Fail(ctx, ch, err)
				return
			}
		}

		if err := stream.Err(); err != nil {
			// Mirror openaicompat/anthropic: keep partial content on a truncated
			// or malformed SSE data: line ("unexpected end of JSON input")
			// instead of failing and discarding what the user already received.
			// The openai-go SDK's SSE reader stores a json.SyntaxError in
			// stream.Err() when a data: line is truncated mid-flight.
			if state.HasContent() && isJSONUnmarshalErr(err) {
				state.AddToolCallsByKey(toolCalls)
				state.EnsureToolUseStopReason()
				state.Finish(ctx, ch)
				return
			}
			state.Fail(ctx, ch, openaicompat.NormalizeAPIError(c.name, err))
			return
		}

		state.AddToolCallsByKey(toolCalls)
		state.EnsureToolUseStopReason()
		// Require a terminal event so a silent EOF mid-SSE is retried instead of
		// surfacing as a clean end_turn (mirrors fantasy's sawTerminalEvent gate).
		state.FinishOrTruncated(ctx, ch, sawTerminal)
	}()

	return ch
}

func retryableResponseError(code responses.ResponseErrorCode) bool {
	return retryableResponseErrorCode(string(code))
}

func retryableResponseErrorCode(code string) bool {
	switch code {
	case string(responses.ResponseErrorCodeServerError),
		string(responses.ResponseErrorCodeRateLimitExceeded),
		string(responses.ResponseErrorCodeVectorStoreTimeout):
		return true
	default:
		return false
	}
}

// ListModels returns the available models for OpenAI using the API
func (c *Client) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	// The ChatGPT Codex backend advertises its own model catalog; fetch it
	// (with a static fallback) rather than the standard /v1/models list.
	if c.subscription {
		return c.subscriptionCatalog(ctx)
	}

	// Use OpenAI API to dynamically fetch models
	page, err := c.client.Models.List(ctx)
	if err != nil {
		return nil, err
	}

	models := make([]llm.ModelInfo, 0, len(page.Data))

	for _, m := range page.Data {
		id := m.ID
		// Skip models that don't support chat completions or responses API
		if strings.HasPrefix(id, "dall-e") ||
			strings.HasPrefix(id, "tts-") ||
			strings.HasPrefix(id, "whisper-") ||
			strings.HasPrefix(id, "text-embedding") ||
			strings.HasPrefix(id, "omni-moderation") ||
			strings.HasPrefix(id, "davinci") ||
			strings.HasPrefix(id, "babbage") ||
			strings.HasPrefix(id, "sora") ||
			strings.HasPrefix(id, "gpt-image") ||
			strings.Contains(id, "-tts") ||
			strings.Contains(id, "-transcribe") ||
			strings.Contains(id, "-realtime") ||
			strings.Contains(id, "computer-use") ||
			strings.HasSuffix(id, "-instruct") {
			continue
		}

		models = append(models, openAIModelInfo(id))
	}

	slices.SortFunc(models, func(a, b llm.ModelInfo) int { return cmp.Compare(a.ID, b.ID) })

	return models, nil
}

func responseMessageParam(role responses.EasyInputMessageRole, msg core.Message) *responses.EasyInputMessageParam {
	param := &responses.EasyInputMessageParam{Role: role}
	if len(msg.Images) == 0 {
		param.Content = responses.EasyInputMessageContentUnionParam{
			OfString: openai.Opt(msg.Content),
		}
		return param
	}

	content := make(responses.ResponseInputMessageContentListParam, 0, len(msg.Images)+1)
	if parts := core.InterleavedContentParts(msg); parts != nil {
		for _, p := range parts {
			switch p.Type {
			case core.ContentPartText:
				content = append(content, responses.ResponseInputContentParamOfInputText(p.Text))
			case core.ContentPartImage:
				content = append(content, responseImageContentPart(p.Image.MediaType, p.Image.Data))
			}
		}
	} else {
		for _, img := range msg.Images {
			content = append(content, responseImageContentPart(img.MediaType, img.Data))
		}
		if msg.Content != "" {
			content = append(content, responses.ResponseInputContentParamOfInputText(msg.Content))
		}
	}
	param.Content = responses.EasyInputMessageContentUnionParam{
		OfInputItemContentList: content,
	}
	return param
}

func responseImageContentPart(mediaType, data string) responses.ResponseInputContentUnionParam {
	part := responses.ResponseInputContentParamOfInputImage(responses.ResponseInputImageDetailAuto)
	if part.OfInputImage != nil {
		part.OfInputImage.ImageURL = openai.String(fmt.Sprintf("data:%s;base64,%s", mediaType, data))
	}
	return part
}

func messageHasResponseContent(msg core.Message) bool {
	return strings.TrimSpace(msg.Content) != "" || len(msg.Images) > 0
}

// reasoningInputParam builds an input reasoning item that echoes a prior
// reasoning block back to the stateless ChatGPT backend.
func reasoningInputParam(r core.ReasoningItem) *responses.ResponseReasoningItemParam {
	p := &responses.ResponseReasoningItemParam{
		ID:      r.ID,
		Summary: []responses.ResponseReasoningItemSummaryParam{},
	}
	if r.EncryptedContent != "" {
		p.EncryptedContent = openai.Opt(r.EncryptedContent)
	}
	if r.Summary != "" {
		p.Summary = []responses.ResponseReasoningItemSummaryParam{{Text: r.Summary}}
	}
	return p
}

// extractReasoning pulls reasoning items (id + encrypted content + summary) from
// a completed response's output, for echoing back on the next stateless turn.
// Items without encrypted content are skipped — they can't be replayed.
func extractReasoning(output []responses.ResponseOutputItemUnion) []core.ReasoningItem {
	var items []core.ReasoningItem
	for _, item := range output {
		if item.Type != "reasoning" {
			continue
		}
		r := item.AsReasoning()
		if r.EncryptedContent == "" {
			continue
		}
		var summary strings.Builder
		for i, s := range r.Summary {
			if i > 0 {
				summary.WriteString("\n\n") // keep parts separated, as in the stream
			}
			summary.WriteString(s.Text)
		}
		items = append(items, core.ReasoningItem{
			ID:               r.ID,
			EncryptedContent: r.EncryptedContent,
			Summary:          summary.String(),
		})
	}
	return items
}

// Ensure Client implements Provider
var _ llm.Provider = (*Client)(nil)

// isJSONUnmarshalErr reports whether err is a JSON parsing failure such as
// "unexpected end of JSON input". The openai-go SDK's Responses SSE reader
// stores this in stream.Err() when a data: line is truncated mid-flight. We
// detect it via errors.As against *json.SyntaxError and also by message
// substring (the SDK may wrap the error in its own type that doesn't satisfy
// errors.As).
func isJSONUnmarshalErr(err error) bool {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(e.Error(), "unexpected end of JSON input") {
			return true
		}
	}
	return false
}
