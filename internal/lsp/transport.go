package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/log"
	"github.com/boytegar/packboy-builder/internal/proc"
)

// maxContentLength guards against unbounded header-triggered allocations.
const maxContentLength = 256 * 1024 * 1024

// FrameReader reads Content-Length framed messages from an io.Reader.
// LSP uses HTTP-style headers ("Content-Length: N\r\n\r\n") followed by a
// JSON body; unlike MCP's NDJSON framing this allows embedded newlines.
type FrameReader struct {
	br *bufio.Reader
}

func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{br: bufio.NewReaderSize(r, 64*1024)}
}

// ReadMessage blocks until the next complete message or an error.
// Returns io.EOF when the stream is cleanly done.
func (f *FrameReader) ReadMessage() ([]byte, error) {
	contentLength := -1
	for {
		line, err := f.br.ReadString('\n')
		if err != nil {
			if err == io.EOF && contentLength >= 0 {
				return nil, fmt.Errorf("lsp: unexpected EOF mid-message")
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("lsp: malformed header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("lsp: invalid Content-Length %q", value)
			}
			if n > maxContentLength {
				return nil, fmt.Errorf("lsp: Content-Length %d exceeds limit %d", n, maxContentLength)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("lsp: missing Content-Length header")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(f.br, buf); err != nil {
		return nil, fmt.Errorf("lsp: reading body: %w", err)
	}
	return buf, nil
}

// lspMessage is a JSON-RPC message that may be a request, response, or
// notification. ID is omitted for notifications and responses; when present
// it is matched against pending requests.
type lspMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *float64        `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *lspError       `json:"error,omitempty"`
}

type lspError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Client is a single LSP server subprocess over stdio with Content-Length
// framing. It is owned by Manager and not safe for concurrent use.
type Client struct {
	config ServerConfig

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *FrameReader

	mu           sync.Mutex
	pending      map[int]*pendingRequest
	notify       NotificationHandler
	alive        bool
	exitCh       chan struct{}
	exitOnce     sync.Once
	readDone     chan struct{}
	openVersions map[string]int
	capabilities ServerCapabilities
	positionEnc  string
}

type pendingRequest struct {
	ch   chan lspMessage
	done <-chan struct{}
}

// NotificationHandler receives server-initiated notifications by method;
// params is the raw JSON payload.
type NotificationHandler func(method string, params json.RawMessage)

func NewClient(config ServerConfig) *Client {
	return &Client{
		config:   config,
		pending:  make(map[int]*pendingRequest),
		exitCh:   make(chan struct{}),
		readDone: make(chan struct{}),
	}
}

// Start launches the subprocess and begins reading messages. It does not
// perform the LSP initialize handshake; callers do that with Send.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.alive {
		c.mu.Unlock()
		return fmt.Errorf("lsp: client for %q already started", c.config.Command)
	}
	c.mu.Unlock()

	cmd := exec.Command(c.config.Command, c.config.Args...)
	cmd.Env = mergeEnv(nil)
	proc.SetProcessGroup(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("lsp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("lsp: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("lsp: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("lsp: failed to start %q: %w", c.config.Command, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.reader = NewFrameReader(stdout)

	go drainStderr(stderr)

	c.mu.Lock()
	c.alive = true
	c.mu.Unlock()

	go c.readMessages()

	return nil
}

// Send performs a JSON-RPC request and waits for its response. The response
// is returned either as a result or as an error message.
func (c *Client) Send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	if !c.alive {
		c.mu.Unlock()
		return nil, fmt.Errorf("lsp: client for %q is not running", c.config.Command)
	}
	req := lspMessage{
		JSONRPC: "2.0",
		ID:      float64Ptr(nextRequestID()),
		Method:  method,
	}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			c.mu.Unlock()
			return nil, fmt.Errorf("lsp: marshal params: %w", err)
		}
		req.Params = b
	}
	id := int(*req.ID)
	pr := &pendingRequest{ch: make(chan lspMessage, 1)}
	c.pending[id] = pr
	c.mu.Unlock()

	if err := c.writeMessage(ctx, req); err != nil {
		c.cancelPending(id)
		return nil, err
	}

	select {
	case msg := <-pr.ch:
		if msg.Error != nil {
			return nil, fmt.Errorf("lsp %s: %s", method, msg.Error.Message)
		}
		return msg.Result, nil
	case <-ctx.Done():
		c.cancelPending(id)
		return nil, ctx.Err()
	case <-c.exitCh:
		c.cancelPending(id)
		return nil, fmt.Errorf("lsp: server %q exited during request %s", c.config.Command, method)
	}
}

func (c *Client) cancelPending(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// Notify sends a fire-and-forget notification (didOpen, didChange, ...).
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	c.mu.Lock()
	if !c.alive {
		c.mu.Unlock()
		return fmt.Errorf("lsp: client for %q is not running", c.config.Command)
	}
	c.mu.Unlock()

	msg := lspMessage{JSONRPC: "2.0", Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("lsp: marshal params: %w", err)
		}
		msg.Params = b
	}
	return c.writeMessage(ctx, msg)
}

func (c *Client) SetNotificationHandler(h NotificationHandler) {
	c.mu.Lock()
	c.notify = h
	c.mu.Unlock()
}

func (c *Client) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive
}

// setPositionEncoding stores the negotiated offset encoding.
func (c *Client) setPositionEncoding(enc string) {
	c.mu.Lock()
	c.positionEnc = enc
	c.mu.Unlock()
}

// Restart kills the current subprocess and re-creates a fresh Client with
// the same config. The caller must re-initialize and reopen files. Returns
// the new client or an error.
func (c *Client) Restart(ctx context.Context) (*Client, error) {
	c.Close()
	newClient := NewClient(c.config)
	newClient.SetNotificationHandler(c.notify)
	if err := newClient.Start(ctx); err != nil {
		return nil, err
	}
	return newClient, nil
}

// Close performs a best-effort graceful shutdown: send the "shutdown"
// request (short deadline), then "exit", then wait for the process, falling
// back to SIGTERM/SIGKILL after bounded timeouts. Safe to call multiple times.
func (c *Client) Close() {
	c.mu.Lock()
	if !c.alive {
		c.mu.Unlock()
		return
	}
	c.alive = false
	cmd := c.cmd
	stdin := c.stdin
	c.mu.Unlock()

	if stdin != nil {
		// Best-effort: ask the server to shut down and exit.
		// Bypass the alive check (we just set it false) by calling
		// writeMessage directly.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		shutdownMsg := lspMessage{JSONRPC: "2.0", ID: float64Ptr(nextRequestID()), Method: "shutdown"}
		_ = c.writeMessage(shutdownCtx, shutdownMsg)
		cancel()
		exitCtx, cancelExit := context.WithTimeout(context.Background(), 2*time.Second)
		exitMsg := lspMessage{JSONRPC: "2.0", Method: "exit"}
		_ = c.writeMessage(exitCtx, exitMsg)
		cancelExit()
		_ = stdin.Close()
	}

	// Wait for the reader goroutine to finish before calling cmd.Wait(),
	// otherwise Wait can race the pipe reads.
	select {
	case <-c.readDone:
	case <-time.After(2 * time.Second):
	}

	wait := make(chan struct{})
	go func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Wait()
		}
		close(wait)
	}()
	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		if cmd != nil && cmd.Process != nil {
			_ = proc.TerminateGroup(cmd, syscall.SIGTERM)
		}
		select {
		case <-wait:
		case <-time.After(3 * time.Second):
			if cmd != nil && cmd.Process != nil {
				_ = proc.TerminateGroup(cmd, syscall.SIGKILL)
				select {
				case <-wait:
				case <-time.After(2 * time.Second):
				}
			}
		}
	}
	c.exitOnce.Do(func() { close(c.exitCh) })
}

func (c *Client) writeMessage(ctx context.Context, msg lspMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("lsp: marshal: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin == nil {
		return fmt.Errorf("lsp: client not started")
	}
	if err := writeFull(ctx, c.stdin, []byte(header)); err != nil {
		return err
	}
	return writeFull(ctx, c.stdin, data)
}

func (c *Client) readMessages() {
	defer close(c.readDone)
	for {
		body, err := c.reader.ReadMessage()
		if err != nil {
			if err != io.EOF {
				log.Logger().Debug("lsp: read error", zap.String("server", c.config.Command), zap.Error(err))
			}
			c.mu.Lock()
			c.alive = false
			c.mu.Unlock()
			c.exitOnce.Do(func() { close(c.exitCh) })
			return
		}

		var msg lspMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			log.Logger().Debug("lsp: unmarshal failed",
				zap.String("server", c.config.Command), zap.Error(err))
			continue
		}

		if msg.ID == nil {
			// Notification (including responses without id are invalid; treat
			// JSON-RPC notifications only).
			c.mu.Lock()
			h := c.notify
			c.mu.Unlock()
			if h != nil {
				h(msg.Method, msg.Params)
			}
			continue
		}

		id := int(*msg.ID)
		c.mu.Lock()
		pr, ok := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if !ok {
			log.Logger().Debug("lsp: response for unknown request id",
				zap.Int("id", id), zap.String("server", c.config.Command))
			continue
		}
		pr.ch <- msg
	}
}

// drainStderr always redirects server stderr into the debug log; inheriting
// os.Stderr would corrupt the Bubble Tea alt-screen.
func drainStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		log.Logger().Debug("lsp stderr", zap.String("line", scanner.Text()))
	}
}

func writeFull(ctx context.Context, w io.Writer, data []byte) error {
	ch := make(chan error, 1)
	go func() {
		_, err := w.Write(data)
		ch <- err
	}()
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

var requestIDAtomic atomic.Int64

func nextRequestID() int {
	return int(requestIDAtomic.Add(1))
}

func float64Ptr(n int) *float64 {
	f := float64(n)
	return &f
}

func mergeEnv(extra map[string]string) []string {
	env := os.Environ()
	if len(extra) == 0 {
		return env
	}
	envMap := make(map[string]string, len(env)+len(extra))
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}
	for k, v := range extra {
		envMap[k] = v
	}
	out := make([]string, 0, len(envMap))
	for k, v := range envMap {
		out = append(out, k+"="+v)
	}
	return out
}
