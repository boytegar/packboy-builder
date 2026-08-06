// Package llmerr classifies LLM provider/stream errors so the agent loop can
// decide whether a failure is worth retrying. It maps each provider SDK's error
// type onto a small, provider-agnostic taxonomy and exposes phase-aware wrapping
// entry points that tag retryable errors with core.RetryableError.
package llmerr

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"
	"google.golang.org/genai"

	"github.com/boytegar/packboy-builder/internal/core"
)

// class is the provider-agnostic failure category.
type class int

const (
	unknown     class = iota // no typed provider or transport signal
	knownFatal               // never retry: cancellation, bad request, auth, not-found, content policy
	retryable                // transient: 408/409, all 5xx (incl. 529), connection/network errors
	rateLimited              // 429 — retry, honoring Retry-After when present
)

// Wrap conservatively classifies an error from a regular/non-streaming
// operation. Unknown errors are returned unchanged and remain fatal.
func Wrap(err error) error {
	return wrap(err, false)
}

// WrapStream classifies a terminal streaming error. Streaming transports can
// lose their typed error at the SDK boundary, so only an otherwise unknown
// terminal error is additionally considered retryable. Known fatal errors keep
// their conservative classification.
func WrapStream(err error) error {
	return wrap(err, true)
}

// MarkRetryable marks a provider error as a known transient failure. Providers
// use this for structured in-band API errors whose retryability would otherwise
// be lost at the generic stream boundary.
func MarkRetryable(err error) error {
	if err == nil {
		return nil
	}
	var retryable core.RetryableError
	if errors.As(err, &retryable) {
		return err
	}
	return retryErr{err: err}
}

// MarkNonRetryable marks a provider error as a known semantic/API failure.
// Providers use this to distinguish in-band API errors from opaque transport
// termination errors without relying on provider error text.
func MarkNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return nonRetryableErr{err: err}
}

func wrap(err error, retryUnknown bool) error {
	if err == nil {
		return nil
	}
	// Cancellation wins over message-based context overflow detection and every
	// retryable transport classification, including when wrapped.
	if errors.Is(err, context.Canceled) {
		return err
	}
	// An overflowed prompt commonly arrives as a typed 400/422. Tag it before
	// status classification so the loop compacts instead of giving up.
	if isContextExceeded(err) {
		return contextErr{err: err}
	}
	switch c, after := classify(err); c {
	case retryable, rateLimited:
		// Both carry a provider hint when one was supplied: 429 sends
		// Retry-After, and 503/overloaded responses often do too.
		return retryErr{err: err, after: after}
	case unknown:
		if retryUnknown {
			return retryErr{err: err}
		}
	}
	return err
}

type nonRetryableErr struct{ err error }

func (e nonRetryableErr) Error() string { return e.err.Error() }
func (e nonRetryableErr) Unwrap() error { return e.err }
func (e nonRetryableErr) nonRetryable() {}

type nonRetryableError interface {
	error
	nonRetryable()
}

// contextExceededSignatures are the ways providers say "this prompt exceeds
// the context window". Matching is on the message text because no provider
// distinguishes it from other 400s with a machine-readable code.
//
// This is the whole safety net for a model whose window Packboy Builder could not size in
// advance: proactive compaction cannot fire without a known limit, so a
// phrasing missing here means the turn fails and keeps failing rather than
// compacting and retrying. Add a provider's wording when adding the provider.
var contextExceededSignatures = []string{
	"prompt is too long",                // Anthropic
	"prompt_too_long",                   // Anthropic (error type)
	"maximum context length",            // OpenAI and OpenAI-compatible
	"context_length_exceeded",           // OpenAI (error code)
	"reduce the length of the messages", // OpenAI (remediation text)
	"input token count",                 // Google Gemini
	"exceeds the maximum number of tokens",
	"context length exceeded",
	"too many tokens",
}

func isContextExceeded(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, sig := range contextExceededSignatures {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

// contextErr satisfies core.ContextExceededError while preserving the original.
type contextErr struct{ err error }

func (e contextErr) Error() string    { return e.err.Error() }
func (e contextErr) Unwrap() error    { return e.err }
func (e contextErr) ContextExceeded() {}

var _ core.ContextExceededError = contextErr{}

// retryErr satisfies core.RetryableError while preserving the original error.
type retryErr struct {
	err   error
	after time.Duration
}

func (e retryErr) Error() string             { return e.err.Error() }
func (e retryErr) Unwrap() error             { return e.err }
func (e retryErr) RetryAfter() time.Duration { return e.after }

var _ core.RetryableError = retryErr{}

// classify maps err onto the taxonomy, returning a Retry-After hint when the
// provider supplied one (429 responses); 0 otherwise.
func classify(err error) (class, time.Duration) {
	// Cancellation is a known fatal classification. Check it explicitly here as
	// well as in wrap so classify cannot collapse it into unknown.
	if errors.Is(err, context.Canceled) {
		return knownFatal, 0
	}
	var nonRetryable nonRetryableError
	if errors.As(err, &nonRetryable) {
		return knownFatal, 0
	}

	// A mid-stream SSE `error` event arrives inside an HTTP 200, so no status
	// code is available — its payload type is the only signal. Check it before
	// the typed-SDK branch: the OpenAI SDK's *ssestream.StreamError is a
	// distinct type from *openai.Error and carries no status.
	if c, ok := classifyStreamError(err); ok {
		return c, 0
	}

	// Provider SDK typed errors carry an HTTP status — the most reliable
	// signal. (openai.Error.Code is a string, so use .StatusCode.)
	var anthErr *anthropic.Error
	if errors.As(err, &anthErr) {
		return fromStatus(anthErr.StatusCode, anthErr.Response)
	}
	var oaiErr *openai.Error
	if errors.As(err, &oaiErr) {
		return fromStatus(oaiErr.StatusCode, oaiErr.Response)
	}
	var gErr genai.APIError
	if errors.As(err, &gErr) {
		// genai.APIError exposes no response headers, so there is no
		// Retry-After to honor — fall back to plain backoff.
		c, _ := fromStatus(gErr.Code, nil)
		return c, 0
	}

	// HTTP/2 stream resets, connection errors, and GOAWAY frames come from the
	// transport, not the application: the request was never judged on its
	// merits, so a fresh connection may well succeed.
	if IsTransportError(err) {
		return retryable, 0
	}

	// Transport-level failures with no HTTP status: connection dropped,
	// refused, reset, or a timeout. All worth a retry.
	if isNetworkError(err) {
		return retryable, 0
	}

	return unknown, 0
}

// fromStatus classifies an HTTP status code, honoring the provider's explicit
// x-should-retry override and extracting a Retry-After hint when present.
func fromStatus(code int, resp *http.Response) (class, time.Duration) {
	// Both the Anthropic and OpenAI SDKs let a provider override status-based
	// retry policy with x-should-retry, and respect it over the status code.
	// Mirror that: the provider knows things the status code cannot express
	// (e.g. a 400 from an overloaded edge, or a permanently-doomed 500).
	switch shouldRetryHeader(resp) {
	case shouldRetryNo:
		return knownFatal, 0
	case shouldRetryYes:
		return retryable, retryAfter(resp)
	case shouldRetryUnset:
		// Fall through to status-based classification.
	}

	switch {
	case code == http.StatusTooManyRequests: // 429
		return rateLimited, retryAfter(resp)
	case code == http.StatusRequestTimeout, // 408
		code == http.StatusConflict, // 409
		code >= 500:                 // all 5xx, incl. Anthropic 529 overloaded
		return retryable, retryAfter(resp)
	default:
		// 400/401/403/404/422, content policy, model-not-found, and
		// context-window-exceeded all land here: retrying cannot help.
		return knownFatal, 0
	}
}

// shouldRetry is the tri-state result of reading the x-should-retry header.
type shouldRetry int

const (
	shouldRetryUnset shouldRetry = iota
	shouldRetryYes
	shouldRetryNo
)

// shouldRetryHeader reads the provider's explicit x-should-retry override.
// Only the exact values "true"/"false" count; anything else is treated as
// absent so a malformed header cannot silently disable retries.
func shouldRetryHeader(resp *http.Response) shouldRetry {
	if resp == nil {
		return shouldRetryUnset
	}
	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("X-Should-Retry"))) {
	case "true":
		return shouldRetryYes
	case "false":
		return shouldRetryNo
	default:
		return shouldRetryUnset
	}
}

// isNetworkError reports whether err is a transport failure worth retrying. A
// dropped/refused/reset connection surfaces as a net.Error (e.g. *net.OpError);
// a mid-stream cutoff surfaces as io.EOF / io.ErrUnexpectedEOF.
//
// net.Error intentionally also matches a per-request timeout
// (context.DeadlineExceeded satisfies net.Error): a request that timed out is
// worth retrying. A user interrupt (context.Canceled) always stays fatal.
func isNetworkError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	// A truncated SSE data: line surfaces as *json.SyntaxError ("unexpected
	// end of JSON input"). Treat it like a mid-stream cutoff (retryable)
	// rather than a fatal provider error, mirroring crush's SSE resilience
	// (crush skips malformed lines; for the SDK path we retry the request).
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// retryAfter parses the provider's retry hint. Headers are tried in order of
// precedence, matching the Anthropic and OpenAI SDKs' own retry logic:
//
//  1. Retry-After-Ms — millisecond precision; the only header that can express
//     a sub-second wait, so a rate limiter's exact reset is not rounded up to
//     a full second.
//  2. Retry-After — delta-seconds or an HTTP-date.
//
// Returns 0 when no header is present or parseable, leaving the caller on its
// own backoff schedule.
func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	if d, ok := retryAfterMs(resp.Header.Get("Retry-After-Ms")); ok {
		return d
	}
	v := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// retryAfterMs parses a Retry-After-Ms header value. ok is false when the
// header is absent or unparseable so the caller falls through to Retry-After;
// a non-positive value is a valid "retry now" and reports ok with 0.
func retryAfterMs(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	ms, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	if ms <= 0 {
		return 0, true
	}
	return time.Duration(ms * float64(time.Millisecond)), true
}
