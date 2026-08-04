package subagent

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/boytegar/packboy-builder/internal/atomicfile"
	"github.com/boytegar/packboy-builder/internal/confdir"
)

// AgentStoreData is the JSON structure for agents.json
type AgentStoreData struct {
	Disabled     []string `json:"disabled"`
	WriteEnabled []string `json:"write_enabled"`
}

// AgentStore handles persistence of agent enabled/disabled states
type AgentStore struct {
	mu           sync.RWMutex
	path         string
	disabled     map[string]bool
	writeEnabled map[string]bool
}

// NewAgentStore creates a new store at the given path
func NewAgentStore(path string) *AgentStore {
	store := &AgentStore{
		path:         path,
		disabled:     make(map[string]bool),
		writeEnabled: make(map[string]bool),
	}
	store.load()
	return store
}

// NewUserAgentStore creates a store for user-level (~/.pcb/agents.json)
func NewUserAgentStore() *AgentStore {
	home, err := os.UserHomeDir()
	if err != nil {
		return &AgentStore{disabled: make(map[string]bool)}
	}
	return NewAgentStore(filepath.Join(confdir.Dir(home), "agents.json"))
}

// NewProjectAgentStore creates a store for project-level (.pcb/agents.json)
func NewProjectAgentStore(cwd string) *AgentStore {
	return NewAgentStore(filepath.Join(confdir.Dir(cwd), "agents.json"))
}

// load reads disabled agents from disk
func (s *AgentStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}

	var storeData AgentStoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		return
	}

	s.disabled = make(map[string]bool)
	for _, name := range storeData.Disabled {
		s.disabled[name] = true
	}
	s.writeEnabled = make(map[string]bool)
	for _, name := range storeData.WriteEnabled {
		s.writeEnabled[name] = true
	}
}

// persistDisabled writes the disabled agent list to disk. Lock-free — operates
// only on the provided snapshot.
func persistDisabled(path string, disabled []string) error {
	return atomicfile.WriteJSON(path, AgentStoreData{Disabled: disabled}, 0o644)
}

// persistWriteEnabled writes the write-enabled agent list to disk. Lock-free —
// operates only on the provided snapshot.
func persistWriteEnabled(path string, writeEnabled []string) error {
	return atomicfile.WriteJSON(path, AgentStoreData{WriteEnabled: writeEnabled}, 0o644)
}

// IsDisabled returns whether an agent is disabled
func (s *AgentStore) IsDisabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.disabled[name]
}

// SetDisabled sets the disabled state for an agent and persists to disk.
func (s *AgentStore) SetDisabled(name string, disabled bool) error {
	s.mu.Lock()
	if disabled {
		s.disabled[name] = true
	} else {
		delete(s.disabled, name)
	}
	// Snapshot while still holding the write lock so no concurrent
	// modification can slip in before we read the state to persist.
	snapshot := make([]string, 0, len(s.disabled))
	for n := range s.disabled {
		snapshot = append(snapshot, n)
	}
	path := s.path
	s.mu.Unlock()

	return persistDisabled(path, snapshot)
}

// GetDisabled returns a copy of the disabled agents map
func (s *AgentStore) GetDisabled() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]bool, len(s.disabled))
	maps.Copy(result, s.disabled)
	return result
}

// IsWriteEnabled returns whether an agent has been granted write permission
// via the runtime store toggle. Default is false (read-only resolution); the
// frontmatter allow_write flag is checked separately by the executor.
func (s *AgentStore) IsWriteEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.writeEnabled[name]
}

// SetWriteEnabled sets the write-enabled state for an agent and persists to disk.
func (s *AgentStore) SetWriteEnabled(name string, enabled bool) error {
	s.mu.Lock()
	if enabled {
		s.writeEnabled[name] = true
	} else {
		delete(s.writeEnabled, name)
	}
	snapshot := make([]string, 0, len(s.writeEnabled))
	for n := range s.writeEnabled {
		snapshot = append(snapshot, n)
	}
	path := s.path
	s.mu.Unlock()

	return persistWriteEnabled(path, snapshot)
}

// GetWriteEnabled returns a copy of the write-enabled agents map
func (s *AgentStore) GetWriteEnabled() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]bool, len(s.writeEnabled))
	maps.Copy(result, s.writeEnabled)
	return result
}
