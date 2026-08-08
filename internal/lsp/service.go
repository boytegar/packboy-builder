package lsp

import (
	"context"
	"sync"
)

// ServiceOptions configures the LSP service at app startup.
type ServiceOptions struct {
	CWD     string
	Servers map[string]ServerConfig
}

// Service is the app-facing handle around the LSP Manager.
type Service struct {
	manager *Manager
}

var (
	defaultMu sync.RWMutex
	svc       *Service
)

// Initialize creates the service and installs it as the default. Idempotent;
// late calls rebuild the manager (e.g. after a cwd change).
func Initialize(opts ServiceOptions) error {
	servers := MergeWithDefaults(opts.Servers)
	m := NewManager(opts.CWD, servers)
	defaultMu.Lock()
	old := svc
	svc = &Service{manager: m}
	defaultMu.Unlock()
	if old != nil {
		old.manager.KillAll()
	}
	return nil
}

// Default returns the installed service, or a zero service if none.
func Default() *Service {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if svc == nil {
		return &Service{manager: NewManager("", nil)}
	}
	return svc
}

// SetDefault installs a specific service (used by tests).
func SetDefault(s *Service) {
	defaultMu.Lock()
	svc = s
	defaultMu.Unlock()
}

// ResetDefault clears the installed service.
func ResetDefault() {
	defaultMu.Lock()
	old := svc
	svc = nil
	defaultMu.Unlock()
	if old != nil {
		old.manager.KillAll()
	}
}

// Manager returns the underlying manager.
func (s *Service) Manager() *Manager {
	return s.manager
}

// Shutdown stops all servers.
func (s *Service) Shutdown() {
	if s == nil || s.manager == nil {
		return
	}
	s.manager.KillAll()
}

// StartServerForPath is a convenience used by tools: it resolves the server
// for a path, starts it, and returns the client.
func (s *Service) StartServerForPath(ctx context.Context, path string) (*Client, string, error) {
	if s == nil || s.manager == nil {
		return nil, "", nil
	}
	name, ok := s.manager.ServerForPath(path)
	if !ok {
		return nil, "", nil
	}
	c, err := s.manager.Start(ctx, name)
	if err != nil {
		return nil, name, err
	}
	return c, name, nil
}
