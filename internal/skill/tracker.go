package skill

import (
	"sort"
	"sync"
)

// Tracker records skills loaded during one agent session. It is intentionally
// in-memory: starting a new session creates a fresh tracker.
type Tracker struct {
	mu     sync.RWMutex
	loaded map[string]struct{}
}

func NewTracker() *Tracker {
	return &Tracker{loaded: make(map[string]struct{})}
}

func (t *Tracker) MarkLoaded(name string) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	t.loaded[name] = struct{}{}
	t.mu.Unlock()
}

func (t *Tracker) IsLoaded(name string) bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	_, ok := t.loaded[name]
	t.mu.RUnlock()
	return ok
}

func (t *Tracker) LoadedNames() []string {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	names := make([]string, 0, len(t.loaded))
	for name := range t.loaded {
		names = append(names, name)
	}
	t.mu.RUnlock()
	sort.Strings(names)
	return names
}
