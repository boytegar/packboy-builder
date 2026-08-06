package llmerr

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/openai/openai-go/v3/packages/ssestream"
)

// TransientStreamErrorTypes are provider error "type" (or "code") values that
// name a temporary server-side condition worth retrying.
//
// A mid-stream SSE error event rides inside an already-successful HTTP 200
// response, so the status code cannot signal retryability — the payload's type
// is the only signal there is. This is the canonical list: providers parse
// their SDK-specific error shapes but defer the transient/permanent policy
// decision here, so a new provider cannot quietly invent its own retry policy.
var TransientStreamErrorTypes = map[string]bool{
	"server_error":     true,
	"internal_error":   true,
	"overloaded_error": true,
	"api_error":        true,
	"rate_limit_error": true,
}

// permanentStreamErrorTypes are in-band error types whose failure is a property
// of the request itself: replaying the identical request reproduces it, so a
// retry only burns the budget and delays the real error.
//
// A type in neither set stays unclassified on purpose. The streaming path then
// keeps its conservative fallback (retry an opaque terminal error), which is
// the right default for a provider wording nobody has catalogued yet.
var permanentStreamErrorTypes = map[string]bool{
	"invalid_request_error": true,
	"authentication_error":  true,
	"permission_error":      true,
	"permission_denied":     true,
	"not_found_error":       true,
	"request_too_large":     true,
	"invalid_api_key":       true,
	"insufficient_quota":    true,
	"account_deactivated":   true,
	"model_not_found":       true,
	"content_filter":        true,
	"invalid_prompt":        true,
	"billing_error":         true,
}

// streamErrorPrefix is the message prefix both the OpenAI and Anthropic SDKs
// use when an SSE `error` event terminates a stream. Anthropic reports it with
// a bare fmt.Errorf and no exported type, so the prefix is the only handle on
// it; OpenAI wraps it in *ssestream.StreamError but formats the message the
// same way.
const streamErrorPrefix = "received error while streaming:"

// streamErrorEnvelope covers both in-band error shapes. The useful detail is
// nested under "error" in each; the outer "type" is the literal "error" and
// carries no classification value.
//
//	Anthropic: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}
//	OpenAI:    {"error":{"type":"server_error","code":"server_error","message":"..."}}
type streamErrorEnvelope struct {
	Error struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// classifyStreamError classifies a mid-stream SSE `error` event by its payload
// type. ok is false when err is not an in-band stream error, or when its type
// is neither known-transient nor known-permanent; the caller then falls
// through to transport-level classification.
func classifyStreamError(err error) (c class, ok bool) {
	payload, found := streamErrorPayload(err)
	if !found {
		return unknown, false
	}
	switch errType := streamErrorType(payload); {
	case errType == "":
		return unknown, false
	case TransientStreamErrorTypes[errType]:
		return retryable, true
	case permanentStreamErrorTypes[errType]:
		return knownFatal, true
	default:
		return unknown, false
	}
}

// streamErrorPayload extracts the raw JSON body of an in-band SSE error event.
// It prefers the OpenAI SDK's typed *ssestream.StreamError, which carries the
// undecorated event data, and falls back to the message prefix shared by both
// SDKs.
func streamErrorPayload(err error) (payload string, ok bool) {
	var streamErr *ssestream.StreamError
	if errors.As(err, &streamErr) {
		if data := strings.TrimSpace(string(streamErr.Event.Data)); data != "" {
			return data, true
		}
		if _, rest, cut := strings.Cut(streamErr.Message, streamErrorPrefix); cut {
			return strings.TrimSpace(rest), true
		}
		// Still an in-band error, but nothing survives to classify it by.
		return "", true
	}
	if _, rest, cut := strings.Cut(err.Error(), streamErrorPrefix); cut {
		return strings.TrimSpace(rest), true
	}
	return "", false
}

// streamErrorType returns the error type of an in-band SSE payload, falling
// back to the error code when no type is present (OpenAI-compatible providers
// populate one or the other). A payload that is not the standard envelope
// stays unclassified rather than being guessed at.
func streamErrorType(payload string) string {
	if payload == "" {
		return ""
	}
	var envelope streamErrorEnvelope
	if json.Unmarshal([]byte(payload), &envelope) != nil {
		return ""
	}
	if envelope.Error.Type != "" {
		return envelope.Error.Type
	}
	return envelope.Error.Code
}
