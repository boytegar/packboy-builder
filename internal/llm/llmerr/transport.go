package llmerr

import "strings"

// http2TransportErrorFragments are message fragments that identify a transient
// HTTP/2 transport failure:
//
//	http2.StreamError     "stream error: stream ID 27; INTERNAL_ERROR; received from peer"
//	http2.ConnectionError "http2: connection error: PROTOCOL_ERROR"
//	http2.GoAwayError     "http2: server sent GOAWAY and closed the connection; ..."
//	transport teardown    "http2: client connection lost"
//
// Matching is on message text rather than error type because Go's standard
// library bundles its own copy of the http2 package whose error types are
// unexported and therefore unreachable via errors.As. The fragments below are
// stable across both the stdlib copy and golang.org/x/net/http2.
//
// The list stays deliberately tight: a fragment that also appears in provider
// application errors would silently turn a permanent failure into a retry loop
// that can never succeed.
var http2TransportErrorFragments = []string{
	// RST_STREAM: INTERNAL_ERROR, REFUSED_STREAM, CANCEL, ...
	"stream error:",
	// Connection-level protocol error.
	"connection error:",
	// Peer draining the connection; the in-flight request never landed.
	"server sent GOAWAY",
	// The transport tore the connection down under an in-flight request.
	"client connection lost",
	"client connection force closed",
}

// IsTransportError reports whether err, or any error in its chain, is a
// transient transport-level failure that is safe to retry on a fresh
// connection. In practice these are HTTP/2 stream resets, connection errors,
// and GOAWAY frames: they originate in the transport, not the application, so
// the request was never judged on its merits and a retry is not a repeat of a
// rejected call.
//
// Wrapped errors embed the inner message, so scanning the top-level string
// covers the whole chain.
func IsTransportError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, fragment := range http2TransportErrorFragments {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}
