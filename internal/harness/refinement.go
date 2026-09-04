package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RefinementAction enumerates the edit operations on harness entries.
type RefinementAction string

const (
	ActionCreate RefinementAction = "create"
	ActionUpdate RefinementAction = "update"
	ActionDelete RefinementAction = "delete"
)

// RefinementEdit is a single proposed change to harness state.
type RefinementEdit struct {
	Action    RefinementAction `json:"action"`
	Kind      RefinementKind   `json:"kind"`
	ID        string           `json:"id,omitempty"`
	Title     string           `json:"title,omitempty"`
	Content   string           `json:"content,omitempty"`
	Path      string           `json:"path,omitempty"`
	Reference map[string]any   `json:"reference,omitempty"`
	Arguments map[string]any   `json:"arguments,omitempty"`
	Metadata  map[string]any   `json:"metadata,omitempty"`
	Reason    string           `json:"reason,omitempty"`
}

// RefinementProposal is the model's refinement plan.
type RefinementProposal struct {
	Summary         string           `json:"summary"`
	Rationale       string           `json:"rationale"`
	Edits           []RefinementEdit `json:"edits"`
	ExpectedOutcome string           `json:"expectedOutcome"`
}

// AppliedRefinementEdit extends RefinementEdit with the result of applying it.
type AppliedRefinementEdit struct {
	RefinementEdit
	ID      string        `json:"id"`
	Before  *HarnessEntry `json:"before,omitempty"`
	After   *HarnessEntry `json:"after,omitempty"`
	Applied bool          `json:"applied"`
	Error   string        `json:"error,omitempty"`
}

// RefinementResult is the outcome of applying a proposal.
type RefinementResult struct {
	ID               string                  `json:"id"`
	Summary          string                  `json:"summary"`
	Rationale        string                  `json:"rationale"`
	ExpectedOutcome  string                  `json:"expectedOutcome"`
	AppliedEdits     []AppliedRefinementEdit `json:"appliedEdits"`
	HarnessStatePath string                  `json:"harnessStatePath"`
	RollbackOf       string                  `json:"rollbackOf,omitempty"`
	Scope            HarnessScope            `json:"scope,omitempty"`
}

// RefineOptions controls refinement behaviour.
type RefineOptions struct {
	Instructions string
	RollbackID   string
	Global       bool
}

// ValidateEdit returns an error string if the edit is invalid, or nil if valid.
func ValidateEdit(edit RefinementEdit, computedID string) string {
	switch edit.Action {
	case ActionCreate, ActionUpdate, ActionDelete:
	default:
		return fmt.Sprintf("unsupported action %s", string(edit.Action))
	}

	switch edit.Kind {
	case KindPrompt, KindMemory, KindSkill, KindSubagent:
	default:
		return fmt.Sprintf("unsupported kind %s", string(edit.Kind))
	}

	// The base system prompt is immutable.
	if edit.Kind == KindPrompt && (edit.ID == "base_system_prompt" || computedID == "base_system_prompt") {
		return "base system prompt is not editable"
	}

	if edit.Action != ActionCreate && edit.ID == "" {
		return fmt.Sprintf("%s requires id", string(edit.Action))
	}

	if edit.Action != ActionDelete && (edit.Title == "" || edit.Content == "") {
		return fmt.Sprintf("%s requires title and content", string(edit.Action))
	}

	// Skill edits must have a Python reference and arguments contract.
	if edit.Action != ActionDelete && edit.Kind == KindSkill {
		if edit.Arguments == nil {
			return fmt.Sprintf("%s skill requires arguments", string(edit.Action))
		}
		ref := edit.Reference
		if ref == nil {
			return fmt.Sprintf("%s skill requires python reference", string(edit.Action))
		}
		if ref["type"] != "python" {
			return fmt.Sprintf("%s skill reference.type must be python", string(edit.Action))
		}
		hasImport := ref["import"] != nil && ref["import"] != ""
		hasCallable := ref["callable"] != nil && ref["callable"] != ""
		if !hasImport {
			return fmt.Sprintf("%s skill requires python import", string(edit.Action))
		}
		if !hasCallable {
			return fmt.Sprintf("%s skill requires callable or call_pattern", string(edit.Action))
		}
	}

	return ""
}

// ApplyRefinementProposal applies a proposal to a harness state, returning the
// result with per-edit application status. The state is mutated in place.
// If opts.RollbackID is set, the result is marked as a rollback of that ID.
func ApplyRefinementProposal(
	st *HarnessState,
	proposal RefinementProposal,
	opts struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	},
) *RefinementResult {
	result := &RefinementResult{
		ID:              opts.ID,
		Summary:         proposal.Summary,
		Rationale:       proposal.Rationale,
		ExpectedOutcome: proposal.ExpectedOutcome,
		Scope:           opts.Scope,
		RollbackOf:      opts.RollbackOf,
	}

	if st.Entries == nil {
		st.Entries = make(map[RefinementKind]map[string]HarnessEntry)
		for _, k := range AllKinds() {
			if st.Entries[k] == nil {
				st.Entries[k] = make(map[string]HarnessEntry)
			}
		}
	}

	for _, edit := range proposal.Edits {
		applied := AppliedRefinementEdit{RefinementEdit: edit}

		// Compute ID for create if not provided.
		computedID := edit.ID
		if computedID == "" && edit.Action == ActionCreate {
			computedID = Slug(edit.Title, string(edit.Kind))
		}
		applied.ID = computedID

		// Validate.
		if errMsg := ValidateEdit(edit, computedID); errMsg != "" {
			applied.Applied = false
			applied.Error = errMsg
			result.AppliedEdits = append(result.AppliedEdits, applied)
			continue
		}

		records := st.Entries[edit.Kind]
		if records == nil {
			records = make(map[string]HarnessEntry)
			st.Entries[edit.Kind] = records
		}

		before := cloneEntry(records[computedID])
		applied.Before = before

		switch edit.Action {
		case ActionCreate, ActionUpdate:
			now := NowISO()
			existing := records[computedID]
			entry := HarnessEntry{
				ID:        computedID,
				Kind:      edit.Kind,
				Title:     edit.Title,
				Content:   edit.Content,
				Path:      edit.Path,
				Scope:     opts.Scope,
				Reference: edit.Reference,
				Arguments: edit.Arguments,
				Metadata:  edit.Metadata,
				Source:    "refine",
				CreatedAt: now,
				UpdatedAt: now,
				Version:   1,
			}
			if existing.ID != "" {
				entry.CreatedAt = existing.CreatedAt
				entry.Version = existing.Version + 1
			}
			if entry.Reference == nil {
				entry.Reference = map[string]any{}
			}
			if entry.Arguments == nil {
				entry.Arguments = map[string]any{}
			}
			if entry.Metadata == nil {
				entry.Metadata = map[string]any{}
			}
			records[computedID] = entry
			after := entry
			applied.After = &after
			applied.Applied = true

		case ActionDelete:
			delete(records, computedID)
			applied.Applied = true
		}

		result.AppliedEdits = append(result.AppliedEdits, applied)
	}

	return result
}

// AppendRefinementHistory appends a completed refinement result to the
// JSONL history log. This enables rollback from any session.
func AppendRefinementHistory(harnessStateDir string, result *RefinementResult) (string, error) {
	historyPath := RefinementHistoryPath(harnessStateDir)
	if err := os.MkdirAll(harnessStateDir, 0o755); err != nil {
		return "", fmt.Errorf("harness: create dir: %w", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("harness: marshal result: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("harness: open history: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("harness: write history: %w", err)
	}

	return historyPath, nil
}

// LoadRefinementHistory reads the JSONL history log and returns all results.
func LoadRefinementHistory(harnessStateDir string) []*RefinementResult {
	historyPath := RefinementHistoryPath(harnessStateDir)
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil
	}

	var results []*RefinementResult
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r RefinementResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		results = append(results, &r)
	}
	return results
}

// MergeRefinementHistory merges global and session refinement histories,
// de-duplicating by ID. Session entries take precedence.
func MergeRefinementHistory(global, session []*RefinementResult) []*RefinementResult {
	byID := make(map[string]*RefinementResult)
	var order []string

	for _, r := range global {
		byID[r.ID] = r
		order = append(order, r.ID)
	}
	for _, r := range session {
		if _, exists := byID[r.ID]; !exists {
			order = append(order, r.ID)
		}
		byID[r.ID] = r
	}

	var out []*RefinementResult
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

// RollbackResult reverses a previously applied refinement by swapping before/after
// snapshots. If an edit was a create, it is deleted. If it was a delete, the
// before entry is restored. If it was an update, the before entry is restored.
func RollbackResult(st *HarnessState, target *RefinementResult) *RefinementResult {
	rollback := &RefinementResult{
		ID:         NewID(),
		Summary:    fmt.Sprintf("Rollback of %s", target.ID),
		Rationale:  fmt.Sprintf("Reversing refinement %s: %s", target.ID, target.Summary),
		Scope:      target.Scope,
		RollbackOf: target.ID,
	}

	for _, edit := range target.AppliedEdits {
		if !edit.Applied {
			continue
		}

		records := st.Entries[edit.Kind]
		if records == nil {
			continue
		}

		now := NowISO()
		switch edit.Action {
		case ActionCreate:
			// Reverse: delete the created entry.
			delete(records, edit.ID)
			rollback.AppliedEdits = append(rollback.AppliedEdits, AppliedRefinementEdit{
				RefinementEdit: RefinementEdit{
					Action: ActionDelete,
					Kind:   edit.Kind,
					ID:     edit.ID,
					Title:  edit.Title,
				},
				ID:      edit.ID,
				Before:  edit.After,
				Applied: true,
			})

		case ActionUpdate:
			// Reverse: restore the before entry.
			if edit.Before != nil {
				entry := *edit.Before
				entry.UpdatedAt = now
				entry.Version = entry.Version + 1
				records[edit.ID] = entry
				after := entry
				rollback.AppliedEdits = append(rollback.AppliedEdits, AppliedRefinementEdit{
					RefinementEdit: RefinementEdit{
						Action:  ActionUpdate,
						Kind:    edit.Kind,
						ID:      edit.ID,
						Title:   entry.Title,
						Content: entry.Content,
					},
					ID:      edit.ID,
					Before:  edit.After,
					After:   &after,
					Applied: true,
				})
			}

		case ActionDelete:
			// Reverse: restore the deleted entry from before snapshot.
			if edit.Before != nil {
				entry := *edit.Before
				entry.UpdatedAt = now
				entry.Version = entry.Version + 1
				records[edit.ID] = entry
				after := entry
				rollback.AppliedEdits = append(rollback.AppliedEdits, AppliedRefinementEdit{
					RefinementEdit: RefinementEdit{
						Action:  ActionCreate,
						Kind:    edit.Kind,
						ID:      edit.ID,
						Title:   entry.Title,
						Content: entry.Content,
					},
					ID:      edit.ID,
					Before:  nil,
					After:   &after,
					Applied: true,
				})
			}
		}
	}

	rollback.ExpectedOutcome = fmt.Sprintf("Reversed %d edit(s) from refinement %s", len(rollback.AppliedEdits), target.ID)
	return rollback
}

// NormalizeProposal parses an untrusted map (from LLM JSON) into a typed proposal.
func NormalizeProposal(raw map[string]any) RefinementProposal {
	p := RefinementProposal{
		Summary:         getString(raw, "summary", "Refined continual harness state"),
		Rationale:       getString(raw, "rationale", ""),
		ExpectedOutcome: getString(raw, "expectedOutcome", ""),
	}

	if edits, ok := raw["edits"].([]any); ok {
		for _, e := range edits {
			em, ok := e.(map[string]any)
			if !ok {
				continue
			}
			p.Edits = append(p.Edits, RefinementEdit{
				Action:    RefinementAction(getString(em, "action", "")),
				Kind:      RefinementKind(getString(em, "kind", "")),
				ID:        getString(em, "id", ""),
				Title:     getString(em, "title", ""),
				Content:   getString(em, "content", ""),
				Path:      getString(em, "path", ""),
				Reference: toRecord(em["reference"]),
				Arguments: toRecord(em["arguments"]),
				Metadata:  toRecord(em["metadata"]),
				Reason:    getString(em, "reason", ""),
			})
		}
	}

	return p
}

// --- helpers ---

func cloneEntry(e HarnessEntry) *HarnessEntry {
	data, _ := json.Marshal(e)
	var copy HarnessEntry
	_ = json.Unmarshal(data, &copy)
	return &copy
}

func getString(m map[string]any, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
