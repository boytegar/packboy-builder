package session

import (
	"context"
	"strings"
	"time"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/session/transcript"
)

// SearchResult represents a session that matched a search query, with
// the matched snippets and a relevance score.
type SearchResult struct {
	SessionID    string
	Title        string
	LastPrompt   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MessageCount int
	Snippets     []SearchSnippet
	Score        int // higher = more relevant
}

// SearchSnippet is a matched portion of a session's conversation.
type SearchSnippet struct {
	Role      string // "user", "assistant", etc.
	MessageID string
	Text      string // the matched content (truncated to a window around the match)
	MatchTerm string // the term that matched
}

// SearchOptions controls how sessions are searched.
type SearchOptions struct {
	Query       string // the search query (space-separated terms, all must match)
	Limit       int    // max results (0 = default 20)
	MaxSnippets int    // max snippets per session (0 = default 3)
}

const (
	defaultSearchLimit = 20
	defaultMaxSnippets = 3
	maxSnippetLen      = 200
)

// Search searches all sessions in this project for the given query.
// The query is split into terms (space-separated); a session matches
// if all terms appear (case-insensitive) in any of its messages.
// Results are ranked by total match count (more matches = higher score).
func (s *Store) Search(opts SearchOptions) ([]*SearchResult, error) {
	terms := tokenizeSearchQuery(opts.Query)
	if len(terms) == 0 {
		return nil, nil
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	maxSnippets := opts.MaxSnippets
	if maxSnippets <= 0 {
		maxSnippets = defaultMaxSnippets
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items, err := s.transcriptStore.List(context.Background(), s.projectID, transcript.ListOptions{})
	if err != nil {
		return nil, err
	}

	var results []*SearchResult
	for _, item := range items {
		// First check title and lastPrompt (cheap, no file read)
		titleLower := strings.ToLower(item.Title)
		promptLower := strings.ToLower(item.LastPrompt)
		metaScore := 0
		for _, term := range terms {
			if strings.Contains(titleLower, term) {
				metaScore += 2 // title match is worth more
			}
			if strings.Contains(promptLower, term) {
				metaScore += 1
			}
		}

		// Load the session to search its messages
		tx, err := s.transcriptStore.Load(context.Background(), item.SessionID)
		if err != nil || tx == nil {
			continue
		}

		snippets, score := searchMessages(tx.Messages, terms, maxSnippets)
		score += metaScore

		if score == 0 {
			continue // no matches
		}

		results = append(results, &SearchResult{
			SessionID:    item.SessionID,
			Title:        item.Title,
			LastPrompt:   item.LastPrompt,
			CreatedAt:    item.CreatedAt,
			UpdatedAt:    item.UpdatedAt,
			MessageCount: item.MessageCount,
			Snippets:     snippets,
			Score:        score,
		})
	}

	// Sort by score descending, then by updatedAt descending
	sortSearchResults(results)

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// searchMessages searches a session's messages for the given terms.
// Returns up to maxSnippets matching snippets and a total score.
func searchMessages(messages []transcript.Node, terms []string, maxSnippets int) ([]SearchSnippet, int) {
	var snippets []SearchSnippet
	score := 0

	for _, msg := range messages {
		role := msg.Role
		for _, block := range msg.Content {
			if block.Type != "text" {
				continue
			}
			text := block.Text
			textLower := strings.ToLower(text)
			matched := false
			for _, term := range terms {
				idx := strings.Index(textLower, term)
				if idx >= 0 {
					matched = true
					score++
					if len(snippets) < maxSnippets {
						snippet := extractSnippet(text, idx, term)
						snippets = append(snippets, SearchSnippet{
							Role:      role,
							MessageID: msg.ID,
							Text:      snippet,
							MatchTerm: term,
						})
					}
				}
			}
			_ = matched
		}
	}
	return snippets, score
}

// extractSnippet extracts a window of text around the matched term.
func extractSnippet(text string, matchIdx int, term string) string {
	start := matchIdx - maxSnippetLen/2 + len(term)
	if start < 0 {
		start = 0
	}
	end := matchIdx + len(term) + maxSnippetLen/2
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}
	return snippet
}

// tokenizeSearchQuery splits the query into lowercase search terms.
func tokenizeSearchQuery(query string) []string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == ';'
	})
	return fields
}

// sortSearchResults sorts by score (desc), then updatedAt (desc).
func sortSearchResults(results []*SearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if shouldSwap(results[i], results[j]) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func shouldSwap(a, b *SearchResult) bool {
	if a.Score != b.Score {
		return a.Score < b.Score
	}
	return a.UpdatedAt.Before(b.UpdatedAt)
}

// SearchMessagesByText searches a slice of core.Message for the given query.
// This is useful for searching loaded sessions without the transcript store.
func SearchMessagesByText(messages []core.Message, query string, maxSnippets int) ([]SearchSnippet, int) {
	if maxSnippets <= 0 {
		maxSnippets = defaultMaxSnippets
	}
	terms := tokenizeSearchQuery(query)
	if len(terms) == 0 {
		return nil, 0
	}

	var snippets []SearchSnippet
	score := 0

	for _, msg := range messages {
		role := string(msg.Role)
		text := msg.Content
		if text == "" && msg.ToolResult != nil {
			text = msg.ToolResult.Content
		}
		textLower := strings.ToLower(text)
		for _, term := range terms {
			idx := strings.Index(textLower, term)
			if idx >= 0 {
				score++
				if len(snippets) < maxSnippets {
					snippet := extractSnippet(text, idx, term)
					snippets = append(snippets, SearchSnippet{
						Role:      role,
						MessageID: msg.ID,
						Text:      snippet,
						MatchTerm: term,
					})
				}
			}
		}
	}
	return snippets, score
}
