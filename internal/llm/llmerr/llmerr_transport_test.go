package llmerr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/boytegar/packboy-builder/internal/core"
)

// An HTTP/2 reset means the request never reached the application: it was
// killed by the transport. Retrying on a fresh connection is the whole point,
// so both the streaming and non-streaming paths have to recognize it.
func TestIsTransportError(t *testing.T) {
	transport := []struct{ name, msg string }{
		{"stream reset", "stream error: stream ID 11; INTERNAL_ERROR; received from peer"},
		{"refused stream", "stream error: stream ID 5; REFUSED_STREAM"},
		{"connection error", "http2: connection error: PROTOCOL_ERROR"},
		{"goaway", `http2: server sent GOAWAY and closed the connection; LastStreamID=7, ErrCode=NO_ERROR, debug=""`},
		{"connection lost", "http2: client connection lost"},
		{"connection force closed", "http2: client connection force closed via ClientConn.Close"},
	}
	for _, c := range transport {
		t.Run(c.name, func(t *testing.T) {
			err := errors.New(c.msg)
			if !IsTransportError(err) {
				t.Fatalf("IsTransportError(%q) = false, want true", c.msg)
			}
			// Wrapping happens at every layer between the SDK and the agent
			// loop; matching only the bare error would miss every real case.
			wrapped := fmt.Errorf("reading stream: %w", err)
			if !IsTransportError(wrapped) {
				t.Fatalf("IsTransportError(wrapped %q) = false, want true", c.msg)
			}
			if _, retryable := retryInfo(Wrap(err)); !retryable {
				t.Fatalf("Wrap(%q) is not retryable", c.msg)
			}
		})
	}

	// An application error must not be mistaken for a transport reset:
	// retrying an invalid request just burns the retry budget.
	notTransport := []struct{ name, msg string }{
		{"nil", ""},
		{"invalid request", "invalid request: unsupported parameter"},
		{"auth", "invalid api key"},
		{"unrelated stream word", "the stream ended before a stop reason arrived"},
	}
	for _, c := range notTransport {
		t.Run("not/"+c.name, func(t *testing.T) {
			var err error
			if c.msg != "" {
				err = errors.New(c.msg)
			}
			if IsTransportError(err) {
				t.Fatalf("IsTransportError(%q) = true, want false", c.msg)
			}
		})
	}
}

// oaiStreamError builds the typed error the OpenAI SDK produces for an in-band
// SSE `error` event: it carries the raw event payload and no HTTP status.
func oaiStreamError(payload string) *ssestream.StreamError {
	return &ssestream.StreamError{
		Message: "received error while streaming: " + payload,
		Event:   ssestream.Event{Type: "error", Data: []byte(payload)},
	}
}

// anthropicStreamError builds the untyped error the Anthropic SDK produces for
// an in-band SSE `error` event — a bare fmt.Errorf with a known prefix.
func anthropicStreamError(payload string) error {
	return fmt.Errorf("received error while streaming: %s", payload)
}

// A mid-stream SSE error event rides inside an HTTP 200, so the status code
// says "success" while the turn actually failed. Only the payload's type says
// whether a retry can help.
func TestStreamErrorClassification(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		wantRetryable bool
		wantFatal     bool
	}{
		{name: "overloaded", payload: `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, wantRetryable: true},
		{name: "api_error", payload: `{"type":"error","error":{"type":"api_error","message":"Internal server error"}}`, wantRetryable: true},
		{name: "server_error", payload: `{"error":{"type":"server_error","message":"upstream failure"}}`, wantRetryable: true},
		{name: "rate_limit", payload: `{"error":{"type":"rate_limit_error","message":"slow down"}}`, wantRetryable: true},
		{name: "code only", payload: `{"error":{"code":"internal_error","message":"boom"}}`, wantRetryable: true},
		{name: "invalid_request", payload: `{"type":"error","error":{"type":"invalid_request_error","message":"bad tool schema"}}`, wantFatal: true},
		{name: "authentication", payload: `{"error":{"type":"authentication_error","message":"bad key"}}`, wantFatal: true},
		{name: "permission", payload: `{"error":{"type":"permission_error","message":"no access"}}`, wantFatal: true},
		{name: "not_found", payload: `{"error":{"type":"not_found_error","message":"no such model"}}`, wantFatal: true},
		{name: "content_filter code", payload: `{"error":{"code":"content_filter","message":"blocked"}}`, wantFatal: true},
	}

	for _, tc := range tests {
		for _, sdk := range []struct {
			name string
			err  func(string) error
		}{
			{name: "openai typed", err: func(p string) error { return oaiStreamError(p) }},
			{name: "anthropic prefix", err: anthropicStreamError},
		} {
			t.Run(sdk.name+"/"+tc.name, func(t *testing.T) {
				err := sdk.err(tc.payload)

				_, wrapRetryable := retryInfo(Wrap(err))
				if wrapRetryable != tc.wantRetryable {
					t.Fatalf("Wrap retryable = %v, want %v", wrapRetryable, tc.wantRetryable)
				}

				// A known-permanent in-band error must stay fatal even on the
				// streaming path, which otherwise retries opaque terminal
				// errors: the payload already told us a retry cannot help.
				_, streamRetryable := retryInfo(WrapStream(err))
				if streamRetryable == tc.wantFatal {
					t.Fatalf("WrapStream retryable = %v, want %v", streamRetryable, !tc.wantFatal)
				}

				if !errors.Is(Wrap(err), err) {
					t.Fatal("original error is not preserved in chain")
				}
			})
		}
	}
}

// An unrecognized in-band payload must not be forced into either bucket: the
// streaming path keeps its conservative "retry an opaque terminal error"
// fallback, and the non-streaming path stays fatal.
func TestStreamErrorUnknownTypeFallsThrough(t *testing.T) {
	unknownPayloads := []struct{ name, payload string }{
		{"unlisted type", `{"error":{"type":"some_new_error","message":"???"}}`},
		{"no type or code", `{"error":{"message":"???"}}`},
		{"not json", `<html>502 Bad Gateway</html>`},
		{"empty", ``},
	}
	for _, c := range unknownPayloads {
		for _, sdk := range []struct {
			name string
			err  func(string) error
		}{
			{name: "openai typed", err: func(p string) error { return oaiStreamError(p) }},
			{name: "anthropic prefix", err: anthropicStreamError},
		} {
			t.Run(sdk.name+"/"+c.name, func(t *testing.T) {
				err := sdk.err(c.payload)
				if _, retryable := retryInfo(Wrap(err)); retryable {
					t.Fatal("Wrap retryable = true, want false for an unclassifiable payload")
				}
				if _, retryable := retryInfo(WrapStream(err)); !retryable {
					t.Fatal("WrapStream retryable = false, want true (opaque terminal fallback)")
				}
			})
		}
	}
}

// A context overflow reported in-band is still a context overflow: the turn
// loop has to compact, not retry the identical oversized prompt.
func TestStreamErrorContextOverflowStillCompacts(t *testing.T) {
	err := anthropicStreamError(`{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 213423 tokens > 200000 maximum"}}`)
	var exceeded core.ContextExceededError
	if !errors.As(WrapStream(err), &exceeded) {
		t.Fatal("in-band context overflow is not tagged ContextExceededError")
	}
	if _, retryable := retryInfo(WrapStream(err)); retryable {
		t.Fatal("in-band context overflow is retryable; want context-exceeded only")
	}
}

// A provider that explicitly says "don't retry this" knows something the
// status code cannot express. Respect it in both directions.
func TestShouldRetryHeaderOverridesStatus(t *testing.T) {
	tests := []struct {
		name          string
		code          int
		header        string
		wantRetryable bool
	}{
		{name: "500 with x-should-retry false is fatal", code: 500, header: "false"},
		{name: "429 with x-should-retry false is fatal", code: 429, header: "false"},
		{name: "400 with x-should-retry true is retryable", code: 400, header: "true", wantRetryable: true},
		{name: "404 with x-should-retry true is retryable", code: 404, header: "true", wantRetryable: true},
		{name: "mixed case true", code: 400, header: "True", wantRetryable: true},
		{name: "500 with no header stays retryable", code: 500, header: "", wantRetryable: true},
		{name: "400 with no header stays fatal", code: 400, header: ""},
		// A malformed value must not silently disable retries on a 503.
		{name: "garbage header falls back to status", code: 503, header: "maybe", wantRetryable: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hdr := http.Header{}
			if tc.header != "" {
				hdr.Set("X-Should-Retry", tc.header)
			}
			c, _ := fromStatus(tc.code, httpResp(tc.code, hdr))
			gotRetryable := c == retryable || c == rateLimited
			if gotRetryable != tc.wantRetryable {
				t.Fatalf("fromStatus(%d, x-should-retry=%q) retryable = %v, want %v",
					tc.code, tc.header, gotRetryable, tc.wantRetryable)
			}
		})
	}
}

// Retry-After-Ms is the only header that can express a sub-second wait.
// Rounding it up to Retry-After's whole seconds wastes real time on every
// rate-limited request.
func TestRetryAfterMsTakesPrecedence(t *testing.T) {
	tests := []struct {
		name string
		hdr  http.Header
		want time.Duration
	}{
		{
			name: "ms wins over seconds",
			hdr:  http.Header{"Retry-After-Ms": {"250"}, "Retry-After": {"8"}},
			want: 250 * time.Millisecond,
		},
		{name: "ms alone", hdr: http.Header{"Retry-After-Ms": {"1500"}}, want: 1500 * time.Millisecond},
		{name: "fractional ms", hdr: http.Header{"Retry-After-Ms": {"1500.5"}}, want: 1500500 * time.Microsecond},
		{name: "zero ms means retry now", hdr: http.Header{"Retry-After-Ms": {"0"}, "Retry-After": {"8"}}, want: 0},
		{name: "negative ms means retry now", hdr: http.Header{"Retry-After-Ms": {"-5"}, "Retry-After": {"8"}}, want: 0},
		// An unparseable Retry-After-Ms must not shadow a usable Retry-After.
		{name: "garbage ms falls back to seconds", hdr: http.Header{"Retry-After-Ms": {"soon"}, "Retry-After": {"8"}}, want: 8 * time.Second},
		{name: "seconds alone", hdr: http.Header{"Retry-After": {"3"}}, want: 3 * time.Second},
		{name: "no headers", hdr: http.Header{}, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryAfter(httpResp(429, tc.hdr)); got != tc.want {
				t.Fatalf("retryAfter = %v, want %v", got, tc.want)
			}
		})
	}
}

// Retry-After is not exclusive to 429: an overloaded 503 often carries one
// too, and ignoring it means retrying before the provider is ready.
func TestRetryAfterHonoredOnServerErrors(t *testing.T) {
	for _, code := range []int{500, 502, 503, 529, 408, 409} {
		t.Run(fmt.Sprintf("%d", code), func(t *testing.T) {
			hdr := http.Header{"Retry-After-Ms": {"750"}}
			c, after := fromStatus(code, httpResp(code, hdr))
			if c != retryable {
				t.Fatalf("fromStatus(%d) class = %v, want retryable", code, c)
			}
			if after != 750*time.Millisecond {
				t.Fatalf("fromStatus(%d) after = %v, want 750ms", code, after)
			}
		})
	}
}
