package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"testing"
)

func TestFrameReaderReadsContentLengthMessage(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"test","params":{"a":1}}`
	data := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body

	fr := NewFrameReader(bytes.NewReader([]byte(data)))
	got, err := fr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(got) != body {
		t.Fatalf("got %q, want %q", got, body)
	}
}

func TestFrameReaderHandlesEmbeddedNewlines(t *testing.T) {
	body := "{\"message\":\"line1\\nline2\"}"
	data := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body

	fr := NewFrameReader(bytes.NewReader([]byte(data)))
	got, err := fr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(got) != body {
		t.Fatalf("got %q, want %q", got, body)
	}
}

func TestFrameReaderMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	msgs := []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}
	for _, m := range msgs {
		buf.WriteString("Content-Length: " + strconv.Itoa(len(m)) + "\r\n\r\n" + m)
	}

	fr := NewFrameReader(&buf)
	for i, want := range msgs {
		got, err := fr.ReadMessage()
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
		if string(got) != want {
			t.Fatalf("message %d: got %q, want %q", i, got, want)
		}
	}
	if _, err := fr.ReadMessage(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestFrameReaderRejectsOversizedBody(t *testing.T) {
	data := "Content-Length: 999999999\r\n\r\n"
	fr := NewFrameReader(bytes.NewReader([]byte(data)))
	if _, err := fr.ReadMessage(); err == nil {
		t.Fatal("expected error for oversized Content-Length")
	}
}

func TestFrameReaderRejectsMissingHeader(t *testing.T) {
	fr := NewFrameReader(bytes.NewReader([]byte("")))
	if _, err := fr.ReadMessage(); err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	req := lspMessage{
		JSONRPC: "2.0",
		ID:      float64Ptr(7),
		Method:  "initialize",
		Params:  json.RawMessage(`{"x":1}`),
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(b, []byte(`"jsonrpc":"2.0"`)) {
		t.Fatalf("unexpected payload: %s", b)
	}
	if !bytes.Contains(b, []byte(`"id":7`)) {
		t.Fatalf("missing id: %s", b)
	}

	var got lspMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID == nil || int(*got.ID) != 7 {
		t.Fatalf("got id %v, want 7", got.ID)
	}
}

func TestNextRequestIDIncrements(t *testing.T) {
	a := nextRequestID()
	b := nextRequestID()
	if b <= a {
		t.Fatalf("request ids must increase: %d then %d", a, b)
	}
}
