package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boytegar/packboy-builder/internal/core"
)

func TestTokenizeSearchQuery(t *testing.T) {
	terms := tokenizeSearchQuery("hello world")
	if len(terms) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(terms))
	}
	if terms[0] != "hello" || terms[1] != "world" {
		t.Errorf("terms = %v, want [hello world]", terms)
	}
}

func TestTokenizeSearchQueryEmpty(t *testing.T) {
	if tokenizeSearchQuery("") != nil {
		t.Error("expected nil for empty query")
	}
	if tokenizeSearchQuery("   ") != nil {
		t.Error("expected nil for whitespace query")
	}
}

func TestSearchMessagesByText(t *testing.T) {
	messages := []core.Message{
		{Role: core.RoleUser, Content: "How do I fix the database connection error?"},
		{Role: core.RoleAssistant, Content: "The database connection pool is exhausted. Increase max connections."},
		{Role: core.RoleUser, Content: "Thanks, that fixed it."},
	}

	snippets, score := SearchMessagesByText(messages, "database connection", 5)
	if score < 2 {
		t.Errorf("expected score >= 2, got %d", score)
	}
	if len(snippets) == 0 {
		t.Error("expected at least 1 snippet")
	}
}

func TestSearchMessagesByTextNoMatch(t *testing.T) {
	messages := []core.Message{
		{Role: core.RoleUser, Content: "hello world"},
	}
	_, score := SearchMessagesByText(messages, "nonexistent", 5)
	if score != 0 {
		t.Errorf("expected score 0, got %d", score)
	}
}

func TestSearchMessagesByTextSnippetExtraction(t *testing.T) {
	longText := "This is a long message that contains the word TARGET somewhere in the middle of the text for testing snippet extraction."
	messages := []core.Message{
		{Role: core.RoleUser, Content: longText},
	}
	snippets, score := SearchMessagesByText(messages, "target", 5)
	if score != 1 {
		t.Errorf("expected score 1, got %d", score)
	}
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(snippets))
	}
	if !contains(snippets[0].Text, "TARGET") {
		t.Errorf("snippet should contain the match term, got: %s", snippets[0].Text)
	}
}

func TestExtractSnippet(t *testing.T) {
	text := "abcdefghijklmnopqrstuvwxyz"
	snippet := extractSnippet(text, 10, "klm")
	if snippet == "" {
		t.Error("expected non-empty snippet")
	}
}

func TestSearchMessagesByTextWithToolResult(t *testing.T) {
	messages := []core.Message{
		{Role: core.RoleUser, ToolResult: &core.ToolResult{Content: "error: file not found in /tmp"}},
	}
	_, score := SearchMessagesByText(messages, "file not found", 5)
	if score != 3 {
		t.Errorf("expected score 3 (3 terms each matched), got %d", score)
	}
}

func TestSearchStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}

	// Save a session with searchable content
	sess := &Snapshot{
		Metadata: SessionMetadata{
			Title: "Fix database bug",
		},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "The database connection keeps dropping"},
			{Role: core.RoleAssistant, Content: "Try increasing the connection pool size"},
		},
	}
	sess.Metadata.ID = "test-session-1"
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	results, err := store.Search(SearchOptions{Query: "database", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result")
	}
	if results[0].Title != "Fix database bug" {
		t.Errorf("title = %q, want 'Fix database bug'", results[0].Title)
	}
	if results[0].Score < 1 {
		t.Errorf("expected score >= 1, got %d", results[0].Score)
	}
}

func TestSearchStoreNoMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStoreWithDir(dir)

	sess := &Snapshot{
		Metadata: SessionMetadata{
			Title: "Hello World",
			ID:    "test-session-2",
		},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "hello world"},
		},
	}
	store.Save(sess)

	results, err := store.Search(SearchOptions{Query: "nonexistentterm", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchStoreMultiTerm(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStoreWithDir(dir)

	sess := &Snapshot{
		Metadata: SessionMetadata{
			Title: "Refactor auth module",
			ID:    "test-session-3",
		},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Please refactor the authentication module to use JWT"},
			{Role: core.RoleAssistant, Content: "I'll update the auth module to use JWT tokens instead of sessions."},
		},
	}
	store.Save(sess)

	results, err := store.Search(SearchOptions{Query: "refactor auth", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'refactor auth'")
	}
}

func TestSearchStoreTitleMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStoreWithDir(dir)

	sess := &Snapshot{
		Metadata: SessionMetadata{
			Title: "Deploy to production",
			ID:    "test-session-4",
		},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "run the deploy script"},
		},
	}
	store.Save(sess)

	results, _ := store.Search(SearchOptions{Query: "production", Limit: 10})
	if len(results) == 0 {
		t.Fatal("expected title match to produce a result")
	}
}

func TestSearchOptionsDefaults(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStoreWithDir(dir)

	sess := &Snapshot{
		Metadata: SessionMetadata{
			Title: "test",
			ID:    "test-session-5",
		},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "searchable content here"},
		},
	}
	store.Save(sess)

	// Empty options should use defaults
	results, err := store.Search(SearchOptions{Query: "searchable"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// Ensure search doesn't panic on empty store
func TestSearchEmptyStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	_ = os.MkdirAll(dir, 0o755)
	store, _ := NewStoreWithDir(dir)

	results, err := store.Search(SearchOptions{Query: "anything"})
	if err != nil {
		t.Fatalf("Search on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results))
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
