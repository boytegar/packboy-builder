package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmptyHarnessState(t *testing.T) {
	st := EmptyHarnessState()
	if st.Schema != 1 {
		t.Fatalf("Schema = %d, want 1", st.Schema)
	}
	for _, kind := range AllKinds() {
		if st.Entries[kind] == nil {
			t.Fatalf("Entries[%s] is nil", kind)
		}
		if len(st.Entries[kind]) != 0 {
			t.Fatalf("Entries[%s] not empty", kind)
		}
	}
}

func TestSaveAndLoadHarnessState(t *testing.T) {
	dir := t.TempDir()

	st := EmptyHarnessState()
	st.Entries[KindMemory]["test-memory"] = HarnessEntry{
		ID:        "test-memory",
		Kind:      KindMemory,
		Title:     "Test Memory",
		Content:   "This is a test memory entry",
		Scope:     ScopeLocal,
		Version:   1,
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-01T00:00:00Z",
	}

	path, err := SaveHarnessState(dir, st)
	if err != nil {
		t.Fatalf("SaveHarnessState: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file not found: %v", err)
	}

	loaded := LoadHarnessState(dir, ScopeLocal)
	if loaded.Schema != 1 {
		t.Fatalf("loaded Schema = %d, want 1", loaded.Schema)
	}

	entry, ok := loaded.Entries[KindMemory]["test-memory"]
	if !ok {
		t.Fatal("entry not found after load")
	}
	if entry.Title != "Test Memory" {
		t.Fatalf("Title = %q, want %q", entry.Title, "Test Memory")
	}
	if entry.Scope != ScopeLocal {
		t.Fatalf("Scope = %q, want %q", entry.Scope, ScopeLocal)
	}
}

func TestLoadHarnessStateMissing(t *testing.T) {
	dir := t.TempDir()
	st := LoadHarnessState(dir, ScopeGlobal)
	if len(st.Entries[KindMemory]) != 0 {
		t.Fatal("expected empty state for missing dir")
	}
}

func TestMergeHarnessStates(t *testing.T) {
	global := EmptyHarnessState()
	global.Entries[KindMemory]["global-note"] = HarnessEntry{
		ID:      "global-note",
		Kind:     KindMemory,
		Title:    "Global Note",
		Content:  "A global memory",
		Scope:    ScopeGlobal,
		Version:  1,
	}

	local := EmptyHarnessState()
	local.Entries[KindMemory]["local-note"] = HarnessEntry{
		ID:      "local-note",
		Kind:     KindMemory,
		Title:    "Local Note",
		Content:  "A local memory",
		Scope:    ScopeLocal,
		Version:  1,
	}
	local.Entries[KindMemory]["global-note"] = HarnessEntry{
		ID:      "global-note",
		Kind:     KindMemory,
		Title:    "Overridden Global",
		Content:  "A local override of a global memory",
		Scope:    ScopeLocal,
		Version:  2,
	}

	merged := MergeHarnessStates(global, local)

	// Global entry should be preserved.
	g, ok := merged.Entries[KindMemory]["global-note"]
	if !ok {
		t.Fatal("global entry not found")
	}
	if g.Scope != ScopeGlobal {
		t.Fatalf("global entry scope = %q, want %q", g.Scope, ScopeGlobal)
	}

	// Local entry should shadow with prefix.
	l, ok := merged.Entries[KindMemory]["local:global-note"]
	if !ok {
		t.Fatal("shadowed local entry not found")
	}
	if l.Scope != ScopeLocal {
		t.Fatalf("shadowed entry scope = %q, want %q", l.Scope, ScopeLocal)
	}

	// Non-colliding local entry should be under bare ID.
	nl, ok := merged.Entries[KindMemory]["local-note"]
	if !ok {
		t.Fatal("non-colliding local entry not found")
	}
	if nl.Scope != ScopeLocal {
		t.Fatalf("local entry scope = %q, want %q", nl.Scope, ScopeLocal)
	}
}

func TestValidateEdit(t *testing.T) {
	tests := []struct {
		name    string
		edit    RefinementEdit
		wantErr bool
	}{
		{
			name: "valid create memory",
			edit: RefinementEdit{
				Action:  ActionCreate,
				Kind:    KindMemory,
				Title:   "Test",
				Content: "Content",
			},
			wantErr: false,
		},
		{
			name: "invalid action",
			edit: RefinementEdit{
				Action:  "invalid",
				Kind:    KindMemory,
				Title:   "Test",
				Content: "Content",
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			edit: RefinementEdit{
				Action:  ActionCreate,
				Kind:    "invalid",
				Title:   "Test",
				Content: "Content",
			},
			wantErr: true,
		},
		{
			name: "base system prompt is immutable",
			edit: RefinementEdit{
				Action:  ActionUpdate,
				Kind:    KindPrompt,
				ID:      "base_system_prompt",
				Title:   "Base",
				Content: "Content",
			},
			wantErr: true,
		},
		{
			name: "delete without id",
			edit: RefinementEdit{
				Action: ActionDelete,
				Kind:   KindMemory,
			},
			wantErr: true,
		},
		{
			name: "create without title",
			edit: RefinementEdit{
				Action:  ActionCreate,
				Kind:    KindMemory,
				Content: "Content",
			},
			wantErr: true,
		},
		{
			name: "skill without reference",
			edit: RefinementEdit{
				Action:   ActionCreate,
				Kind:     KindSkill,
				Title:    "Test Skill",
				Content:  "Content",
				Arguments: map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "valid skill with reference",
			edit: RefinementEdit{
				Action:    ActionCreate,
				Kind:      KindSkill,
				Title:     "Test Skill",
				Content:   "Content",
				Arguments: map[string]any{"query": map[string]any{"type": "string"}},
				Reference: map[string]any{
					"type":     "python",
					"import":   "test_pkg",
					"callable": "test_func",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := ValidateEdit(tt.edit, tt.edit.ID)
			if tt.wantErr && errMsg == "" {
				t.Fatal("expected error, got none")
			}
			if !tt.wantErr && errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
		})
	}
}

func TestApplyRefinementProposal(t *testing.T) {
	st := EmptyHarnessState()

	proposal := RefinementProposal{
		Summary:        "Test refinement",
		Rationale:      "Testing",
		ExpectedOutcome: "Should create a memory",
		Edits: []RefinementEdit{
			{
				Action:  ActionCreate,
				Kind:    KindMemory,
				Title:   "Build Convention",
				Content: "Always run gofmt before committing",
			},
		},
	}

	result := ApplyRefinementProposal(st, proposal, struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	}{
		ID:    "test-refine-1",
		Scope: ScopeLocal,
	})

	if len(result.AppliedEdits) != 1 {
		t.Fatalf("AppliedEdits len = %d, want 1", len(result.AppliedEdits))
	}

	edit := result.AppliedEdits[0]
	if !edit.Applied {
		t.Fatalf("edit not applied: %s", edit.Error)
	}
	if edit.ID == "" {
		t.Fatal("edit ID is empty")
	}

	// Verify entry was actually stored.
	entry, ok := st.Entries[KindMemory][edit.ID]
	if !ok {
		t.Fatal("entry not found in state after apply")
	}
	if entry.Title != "Build Convention" {
		t.Fatalf("Title = %q, want %q", entry.Title, "Build Convention")
	}
	if entry.Version != 1 {
		t.Fatalf("Version = %d, want 1", entry.Version)
	}
	if entry.Scope != ScopeLocal {
		t.Fatalf("Scope = %q, want %q", entry.Scope, ScopeLocal)
	}
}

func TestApplyRefinementProposal_UpdateIncrementsVersion(t *testing.T) {
	st := EmptyHarnessState()

	// First: create.
	createProposal := RefinementProposal{
		Summary: "Create",
		Edits: []RefinementEdit{
			{Action: ActionCreate, Kind: KindMemory, Title: "Test", Content: "v1"},
		},
	}
	result := ApplyRefinementProposal(st, createProposal, struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	}{ID: "test-1", Scope: ScopeLocal})

	entryID := result.AppliedEdits[0].ID

	// Second: update.
	updateProposal := RefinementProposal{
		Summary: "Update",
		Edits: []RefinementEdit{
			{Action: ActionUpdate, Kind: KindMemory, ID: entryID, Title: "Test", Content: "v2"},
		},
	}
	ApplyRefinementProposal(st, updateProposal, struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	}{ID: "test-2", Scope: ScopeLocal})

	entry, ok := st.Entries[KindMemory][entryID]
	if !ok {
		t.Fatal("entry not found after update")
	}
	if entry.Version != 2 {
		t.Fatalf("Version = %d, want 2", entry.Version)
	}
	if entry.Content != "v2" {
		t.Fatalf("Content = %q, want %q", entry.Content, "v2")
	}
}

func TestApplyRefinementProposal_Delete(t *testing.T) {
	st := EmptyHarnessState()

	// Create.
	createProposal := RefinementProposal{
		Edits: []RefinementEdit{
			{Action: ActionCreate, Kind: KindMemory, Title: "Temp", Content: "temporary"},
		},
	}
	result := ApplyRefinementProposal(st, createProposal, struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	}{ID: "test-1", Scope: ScopeLocal})

	entryID := result.AppliedEdits[0].ID

	// Delete.
	deleteProposal := RefinementProposal{
		Edits: []RefinementEdit{
			{Action: ActionDelete, Kind: KindMemory, ID: entryID},
		},
	}
	ApplyRefinementProposal(st, deleteProposal, struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	}{ID: "test-2", Scope: ScopeLocal})

	if _, ok := st.Entries[KindMemory][entryID]; ok {
		t.Fatal("entry still exists after delete")
	}
}

func TestAppendAndLoadRefinementHistory(t *testing.T) {
	dir := t.TempDir()

	result := &RefinementResult{
		ID:         "test-result-1",
		Summary:    "Test refinement",
		Rationale:  "Testing",
		Scope:      ScopeLocal,
		AppliedEdits: []AppliedRefinementEdit{
			{
				RefinementEdit: RefinementEdit{
					Action: ActionCreate,
					Kind:   KindMemory,
					Title:  "Test",
				},
				ID:      "test",
				Applied: true,
			},
		},
	}

	if _, err := AppendRefinementHistory(dir, result); err != nil {
		t.Fatalf("AppendRefinementHistory: %v", err)
	}

	history := LoadRefinementHistory(dir)
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].ID != "test-result-1" {
		t.Fatalf("ID = %q, want %q", history[0].ID, "test-result-1")
	}
}

func TestRollbackResult(t *testing.T) {
	st := EmptyHarnessState()

	// Create an entry via refinement.
	createProposal := RefinementProposal{
		Edits: []RefinementEdit{
			{Action: ActionCreate, Kind: KindMemory, Title: "To Rollback", Content: "will be rolled back"},
		},
	}
	result := ApplyRefinementProposal(st, createProposal, struct {
		ID         string
		RollbackOf string
		Scope      HarnessScope
	}{ID: "original-1", Scope: ScopeLocal})

	entryID := result.AppliedEdits[0].ID

	// Verify entry exists.
	if _, ok := st.Entries[KindMemory][entryID]; !ok {
		t.Fatal("entry not found before rollback")
	}

	// Rollback.
	rollback := RollbackResult(st, result)

	if rollback.RollbackOf != "original-1" {
		t.Fatalf("RollbackOf = %q, want %q", rollback.RollbackOf, "original-1")
	}

	// Entry should be gone.
	if _, ok := st.Entries[KindMemory][entryID]; ok {
		t.Fatal("entry still exists after rollback")
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		input    string
		fallback string
		want     string
	}{
		{"Hello World", "mem", "hello_world"},
		{"Test-Entry 123", "mem", "test_entry_123"},
		{"!!!", "fallback", "fallback"},
		{"", "def", "def"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slug(tt.input, tt.fallback)
			if got != tt.want {
				t.Fatalf("Slug(%q, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestFormatHarnessStateForPrompt(t *testing.T) {
	st := EmptyHarnessState()
	st.Entries[KindMemory]["test"] = HarnessEntry{
		ID:      "test",
		Kind:     KindMemory,
		Title:    "Test Memory",
		Content:  "A test memory entry for validation",
		Scope:    ScopeLocal,
		Version:  1,
	}

	output := FormatHarnessStateForPrompt(st, FormatOptions{})
	if output == "" {
		t.Fatal("output is empty")
	}
	if !contains(output, "Continual Harness State") {
		t.Error("output missing header")
	}
	if !contains(output, "Test Memory") {
		t.Error("output missing entry title")
	}
	if !contains(output, "test") {
		t.Error("output missing entry id")
	}
}

func TestFormatHarnessStateForPromptEmpty(t *testing.T) {
	st := EmptyHarnessState()
	output := FormatHarnessStateForPrompt(st, FormatOptions{})
	if !contains(output, "No saved harness entries") {
		t.Error("output missing empty message")
	}
}

func TestHarnessStatePath(t *testing.T) {
	got := HarnessStatePath(filepath.Join("a", "b"))
	if !contains(got, "harness_state.json") {
		t.Fatalf("path doesn't contain harness_state.json: %s", got)
	}
}

func TestRefinementHistoryPath(t *testing.T) {
	got := RefinementHistoryPath(filepath.Join("a", "b"))
	if !contains(got, "refinements.jsonl") {
		t.Fatalf("path doesn't contain refinements.jsonl: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && len(substr) > 0 && indexOfString(s, substr) >= 0))
}

func indexOfString(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
