package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/log"
)

// Manager owns LSP server clients keyed by configured server name, starting
// them lazily on first use for a given path. It is safe for concurrent use.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	servers map[string]ServerConfig
	diags   map[string][]Diagnostic
	cwd     string
	started map[string]bool // names already attempted (success or failure)
}

func NewManager(cwd string, servers map[string]ServerConfig) *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		servers: servers,
		diags:   make(map[string][]Diagnostic),
		cwd:     cwd,
		started: make(map[string]bool),
	}
}

// ServerForPath picks the configured server whose extension map matches the
// given file path and returns that server's name.
func (m *Manager) ServerForPath(path string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.serverForPathLocked(path)
}

func (m *Manager) serverForPathLocked(path string) (string, bool) {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	for name, cfg := range m.servers {
		for extension := range cfg.ExtensionToLanguage {
			if extension == ext {
				return name, true
			}
		}
	}
	return "", false
}

// Start returns the running client for name, launching it (and blocking until
// initialized) on first use. Failing names are remembered until KillAll, so
// repeated requests don't respawn a broken server.
func (m *Manager) Start(ctx context.Context, name string) (*Client, error) {
	m.mu.RLock()
	if c, ok := m.clients[name]; ok {
		// Crash recovery: if the cached client died, respawn.
		if !c.IsAlive() {
			m.mu.RUnlock()
			return m.restartLocked(ctx, name)
		}
		m.mu.RUnlock()
		return c, nil
	}
	if m.started[name] {
		m.mu.RUnlock()
		return nil, fmt.Errorf("lsp: server %q unavailable (started and failed)", name)
	}
	m.mu.RUnlock()

	cfg, ok := m.servers[name]
	if !ok {
		return nil, fmt.Errorf("lsp: unknown server %q", name)
	}
	if _, err := exec.LookPath(cfg.Command); err != nil {
		m.markFailed(name)
		return nil, fmt.Errorf("lsp: server %q binary %q not found on PATH: %w", name, cfg.Command, err)
	}

	client := NewClient(cfg)
	client.SetNotificationHandler(m.handleNotification(name))

	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Start(startCtx); err != nil {
		m.markFailed(name)
		return nil, err
	}

	if err := m.initialize(startCtx, client, name); err != nil {
		client.Close()
		m.markFailed(name)
		return nil, err
	}

	m.mu.Lock()
	if existing, ok := m.clients[name]; ok {
		m.mu.Unlock()
		client.Close()
		return existing, nil
	}
	m.clients[name] = client
	m.started[name] = true
	m.mu.Unlock()
	return client, nil
}

func (m *Manager) markFailed(name string) {
	m.mu.Lock()
	m.started[name] = true
	m.mu.Unlock()
}

// restartLocked respawns a dead client. Caller must not hold m.mu.
func (m *Manager) restartLocked(ctx context.Context, name string) (*Client, error) {
	if _, ok := m.servers[name]; !ok {
		return nil, fmt.Errorf("lsp: unknown server %q", name)
	}

	// Remove the dead client.
	m.mu.Lock()
	if old, ok := m.clients[name]; ok {
		old.Close()
		delete(m.clients, name)
	}
	// Reset started flag so Start() will attempt a fresh launch.
	m.started[name] = false
	m.mu.Unlock()

	// Re-enter Start (which will see no client and no started flag).
	return m.Start(ctx, name)
}

// RestartServer force-restarts a server by name, even if it appears alive.
// Useful for the lsp_restart tool action and for clearing stale state.
func (m *Manager) RestartServer(ctx context.Context, name string) (*Client, error) {
	m.mu.Lock()
	if old, ok := m.clients[name]; ok {
		old.Close()
		delete(m.clients, name)
	}
	m.started[name] = false
	m.mu.Unlock()
	return m.Start(ctx, name)
}

func (m *Manager) initialize(ctx context.Context, client *Client, name string) error {
	rootURI := ""
	if m.cwd != "" {
		rootURI = "file://" + filepath.ToSlash(m.cwd)
	}
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootPath:  m.cwd,
		RootURI:   rootURI,
		Trace:     "off",
		Capabilities: ClientCapabilities{
			PositionEncodings: []string{"utf-8", "utf-16", "utf-32"},
			TextDocument: TextDocumentClientCapabilities{
				Synchronization: SynchronizationCapabilities{
					DidOpen:   true,
					DidChange: true,
					DidClose:  true,
				},
				Definition:         GenericCapability{},
				References:         GenericCapability{},
				DocumentSymbol:     GenericCapability{},
				Rename:             GenericCapability{},
				PublishDiagnostics: PublishDiagnosticsCapabilities{},
			},
			Workspace: WorkspaceClientCapabilities{},
		},
	}
	result, err := client.Send(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("lsp: initialize %q: %w", name, err)
	}
	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("lsp: initialize %q: bad result: %w", name, err)
	}

	// Negotiate position encoding: prefer utf-8, then utf-16 (LSP default),
	// then utf-32.
	enc := "utf-16"
	switch initResult.Capabilities.PositionEncoding {
	case "utf-8":
		enc = "utf-8"
	case "utf-32":
		enc = "utf-32"
	}
	client.setPositionEncoding(enc)

	_ = client.Notify(ctx, "initialized", struct{}{})
	return nil
}

// handleNotification dispatches server notifications. Only
// textDocument/publishDiagnostics is surfaced; everything else is debug-logged.
func (m *Manager) handleNotification(name string) NotificationHandler {
	return func(method string, params json.RawMessage) {
		switch method {
		case "textDocument/publishDiagnostics":
			var p PublishDiagnosticsParams
			if err := json.Unmarshal(params, &p); err != nil {
				log.Logger().Debug("lsp: bad publishDiagnostics",
					zap.String("server", name), zap.Error(err))
				return
			}
			m.mu.Lock()
			m.diags[p.URI] = p.Diagnostics
			m.mu.Unlock()
		default:
			log.Logger().Debug("lsp: notification",
				zap.String("server", name), zap.String("method", method))
		}
	}
}

// KillAll gracefully stops every running server.
func (m *Manager) KillAll() {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.RUnlock()
	for _, c := range clients {
		c.Close()
	}
}

// ServerStatus is a UI/debug snapshot.
type ServerStatus struct {
	Name  string
	Ready bool
}

func (m *Manager) Status() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerStatus, 0, len(m.servers))
	for name := range m.servers {
		_, ready := m.clients[name]
		out = append(out, ServerStatus{Name: name, Ready: ready})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Diagnostics returns cached diagnostics for a URI, sorted by position.
func (m *Manager) Diagnostics(uri string) []Diagnostic {
	m.mu.RLock()
	d := make([]Diagnostic, len(m.diags[uri]))
	copy(d, m.diags[uri])
	m.mu.RUnlock()
	sort.SliceStable(d, func(i, j int) bool {
		a, b := d[i].Range.Start, d[j].Range.Start
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Character < b.Character
	})
	return d
}

// HasServer reports whether at least one server is configured.
func (m *Manager) HasServer() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.servers) > 0
}
