// Package harness implements the Continual Harness: a persistent, editable layer of
// supplemental prompts, memories, skill descriptions, and subagent specs that the
// agent can refine through small, evidence-backed updates.
//
// The harness is inspired by Prime Agent's Continual Harness (refinement.ts) and
// adapted to Go and the pcb architecture. The base system prompt is immutable;
// harness entries are supplemental only.
package harness

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RefinementKind categorizes harness entries.
type RefinementKind string

const (
	KindPrompt   RefinementKind = "prompt"
	KindMemory   RefinementKind = "memory"
	KindSkill    RefinementKind = "skill"
	KindSubagent RefinementKind = "subagent"
)

// AllKinds returns every valid RefinementKind in canonical order.
func AllKinds() []RefinementKind {
	return []RefinementKind{KindPrompt, KindMemory, KindSkill, KindSubagent}
}

// HarnessScope controls whether an entry is local to the session or global
// (cross-session).
type HarnessScope string

const (
	ScopeLocal  HarnessScope = "local"
	ScopeGlobal HarnessScope = "global"
)

// HarnessEntry is a single editable supplemental state record.
type HarnessEntry struct {
	ID        string         `json:"id"`
	Kind      RefinementKind `json:"kind"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Path      string         `json:"path,omitempty"`
	Scope     HarnessScope   `json:"scope,omitempty"`
	Reference map[string]any `json:"reference,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Source    string         `json:"source,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	Version   int            `json:"version"`
}

// HarnessRefinementEvent records a completed refinement for audit and rollback.
type HarnessRefinementEvent struct {
	ID        string   `json:"id"`
	Trigger   string   `json:"trigger"`
	Changes   []string `json:"changes"`
	Evidence  string   `json:"evidence,omitempty"`
	Outcome   string   `json:"outcome,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// HarnessState is the full persisted harness for a scope (local or global).
type HarnessState struct {
	Schema      int                                        `json:"schema"`
	Entries     map[RefinementKind]map[string]HarnessEntry `json:"entries"`
	Refinements []HarnessRefinementEvent                   `json:"refinements"`
}

// EmptyHarnessState returns a initialised, empty harness state.
func EmptyHarnessState() *HarnessState {
	st := &HarnessState{
		Schema:      1,
		Entries:     make(map[RefinementKind]map[string]HarnessEntry),
		Refinements: nil,
	}
	for _, k := range AllKinds() {
		st.Entries[k] = make(map[string]HarnessEntry)
	}
	return st
}

const (
	// harnessStateFileName is the JSON file holding the full harness state.
	harnessStateFileName = "harness_state.json"
	// harnessDirName is the sub-directory under an agent or session artifact dir.
	harnessDirName = "harness"
	// refinementHistoryFileName is the JSONL log of completed refinements.
	refinementHistoryFileName = "refinements.jsonl"
)

// GlobalHarnessDir returns the directory path for the global (cross-session)
// harness state, rooted under agentDir.
func GlobalHarnessDir(agentDir string) string {
	return filepath.Join(agentDir, harnessDirName)
}

// LocalHarnessDir returns the directory path for the session-scoped harness
// state, rooted under sessionArtifactDir. Returns empty string if
// sessionArtifactDir is empty.
func LocalHarnessDir(sessionArtifactDir string) string {
	if sessionArtifactDir == "" {
		return ""
	}
	return filepath.Join(sessionArtifactDir, harnessDirName)
}

// HarnessStatePath returns the file path for harness_state.json inside
// harnessStateDir.
func HarnessStatePath(harnessStateDir string) string {
	return filepath.Join(harnessStateDir, harnessStateFileName)
}

// RefinementHistoryPath returns the file path for refinements.jsonl inside
// harnessStateDir.
func RefinementHistoryPath(harnessStateDir string) string {
	return filepath.Join(harnessStateDir, refinementHistoryFileName)
}

// LoadHarnessState reads and parses the harness state from harnessStateDir.
// If the file does not exist or is corrupt, an empty state is returned.
// The scope parameter is used to normalise entry scope values.
func LoadHarnessState(harnessStateDir string, scope HarnessScope) *HarnessState {
	statePath := HarnessStatePath(harnessStateDir)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return EmptyHarnessState()
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return EmptyHarnessState()
	}

	st := EmptyHarnessState()

	if v, ok := raw["schema"].(float64); ok {
		st.Schema = int(v)
	}
	if st.Schema == 0 {
		st.Schema = 1
	}

	entriesRaw, _ := raw["entries"].(map[string]any)
	for _, kind := range AllKinds() {
		kindStr := string(kind)
		records, _ := entriesRaw[kindStr].(map[string]any)
		for id, rawEntry := range records {
			entryMap, ok := rawEntry.(map[string]any)
			if !ok {
				continue
			}
			entry := parseHarnessEntry(entryMap, id, kind, scope)
			st.Entries[kind][entry.ID] = entry
		}
	}

	if rawRefs, ok := raw["refinements"].([]any); ok {
		for _, r := range rawRefs {
			if m, ok := r.(map[string]any); ok {
				st.Refinements = append(st.Refinements, parseRefinementEvent(m))
			}
		}
	}

	return st
}

// SaveHarnessState atomically writes the harness state to harnessStateDir.
// It creates the directory if needed and preserves file permissions.
func SaveHarnessState(harnessStateDir string, st *HarnessState) (string, error) {
	statePath := HarnessStatePath(harnessStateDir)
	if err := os.MkdirAll(harnessStateDir, 0o755); err != nil {
		return "", fmt.Errorf("harness: create dir: %w", err)
	}

	mode := os.FileMode(0o600)
	if info, err := os.Stat(statePath); err == nil {
		mode = info.Mode()
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return "", fmt.Errorf("harness: marshal state: %w", err)
	}
	data = append(data, '\n')

	tmpName := fmt.Sprintf(".harness-%s-%s.tmp", nowTimestamp(), randomID(8))
	tmpPath := filepath.Join(harnessStateDir, tmpName)

	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("harness: write temp: %w", err)
	}

	if err := os.Rename(tmpPath, statePath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("harness: rename: %w", err)
	}

	return statePath, nil
}

// MergeHarnessStates combines global and local states into a single read-only
// view. Local entries shadow global entries with the same ID by receiving a
// "local:" prefixed key.
func MergeHarnessStates(global *HarnessState, local *HarnessState) *HarnessState {
	merged := EmptyHarnessState()
	merged.Schema = global.Schema
	if local != nil && local.Schema > merged.Schema {
		merged.Schema = local.Schema
	}

	for _, kind := range AllKinds() {
		// Global entries.
		for id, entry := range global.Entries[kind] {
			e := entry // copy
			if e.Scope == "" {
				e.Scope = ScopeGlobal
			}
			merged.Entries[kind][id] = e
		}
		// Local entries (shadow with prefix if collision).
		if local != nil {
			for id, entry := range local.Entries[kind] {
				e := entry // copy
				if e.Scope == "" {
					e.Scope = ScopeLocal
				}
				key := id
				if _, exists := merged.Entries[kind][key]; exists {
					key = string(ScopeLocal) + ":" + id
				}
				merged.Entries[kind][key] = e
			}
		}
	}

	merged.Refinements = append(merged.Refinements, global.Refinements...)
	if local != nil {
		merged.Refinements = append(merged.Refinements, local.Refinements...)
	}

	return merged
}

// SortedEntries returns entries of the given kind sorted by path, title, id.
func SortedEntries(st *HarnessState, kind RefinementKind) []HarnessEntry {
	entries := make([]HarnessEntry, 0, len(st.Entries[kind]))
	for _, e := range st.Entries[kind] {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		ki := entries[i].Path + "\x00" + entries[i].Title + "\x00" + entries[i].ID
		kj := entries[j].Path + "\x00" + entries[j].Title + "\x00" + entries[j].ID
		return ki < kj
	})
	return entries
}

// --- internal helpers ---

func parseHarnessEntry(m map[string]any, id string, kind RefinementKind, defaultScope HarnessScope) HarnessEntry {
	e := HarnessEntry{
		ID:   id,
		Kind: kind,
	}
	if v, ok := m["title"].(string); ok {
		e.Title = v
	}
	if v, ok := m["content"].(string); ok {
		e.Content = v
	}
	if v, ok := m["path"].(string); ok {
		e.Path = v
	}
	if v, ok := m["scope"].(string); ok {
		e.Scope = HarnessScope(v)
	}
	if e.Scope == "" {
		e.Scope = defaultScope
	}
	if v, ok := m["source"].(string); ok {
		e.Source = v
	}
	if v, ok := m["created_at"].(string); ok {
		e.CreatedAt = v
	}
	if v, ok := m["updated_at"].(string); ok {
		e.UpdatedAt = v
	}
	if v, ok := m["version"].(float64); ok {
		e.Version = int(v)
	}
	e.Reference = toRecord(m["reference"])
	e.Arguments = toRecord(m["arguments"])
	e.Metadata = toRecord(m["metadata"])
	return e
}

func parseRefinementEvent(m map[string]any) HarnessRefinementEvent {
	ev := HarnessRefinementEvent{}
	if v, ok := m["id"].(string); ok {
		ev.ID = v
	}
	if v, ok := m["trigger"].(string); ok {
		ev.Trigger = v
	}
	if v, ok := m["evidence"].(string); ok {
		ev.Evidence = v
	}
	if v, ok := m["outcome"].(string); ok {
		ev.Outcome = v
	}
	if v, ok := m["created_at"].(string); ok {
		ev.CreatedAt = v
	}
	if arr, ok := m["changes"].([]any); ok {
		for _, c := range arr {
			if s, ok := c.(string); ok {
				ev.Changes = append(ev.Changes, s)
			}
		}
	}
	return ev
}

func toRecord(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func nowTimestamp() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewID returns a new unique refinement ID.
func NewID() string {
	return nowTimestamp() + "-" + randomID(12)
}

// Slug normalises a title to a stable id slug.
func Slug(raw, fallback string) string {
	var b []byte
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b = append(b, byte(r))
		case r >= 'A' && r <= 'Z':
			b = append(b, byte(r+32))
		default:
			b = append(b, '_')
		}
	}
	s := string(b)
	for len(s) > 0 && s[0] == '_' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == '_' {
		s = s[:len(s)-1]
	}
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		return fallback
	}
	return s
}

// NowISO returns the current time in RFC 3339 format. Exported for testability.
func NowISO() string { return nowISO() }
