package fuse

import (
	"testing"
	"time"

	"github.com/boytegar/packboy-builder/internal/graph"
)

func TestPrefixBlocker(t *testing.T) {
	b := PrefixBlocker{PrefixLen: 2}
	c1 := EntityCandidate{Label: "Southeast University", Type: "Org"}
	c2 := EntityCandidate{Label: "Southeastern College", Type: "Org"}
	c3 := EntityCandidate{Label: "Stanford University", Type: "Org"}

	if b.Block(c1) != b.Block(c2) {
		t.Error("Southeast and Southeastern should share a block")
	}
	if b.Block(c1) == b.Block(c3) {
		t.Error("Southeast and Stanford should not share a block")
	}
}

func TestTypeBlocker(t *testing.T) {
	b := TypeBlocker{PrefixLen: 2}
	c1 := EntityCandidate{Label: "Alice", Type: "Person"}
	c2 := EntityCandidate{Label: "Alice", Type: "Company"}

	if b.Block(c1) == b.Block(c2) {
		t.Error("same name, different types should be in different blocks")
	}
}

func TestStringSimilarity(t *testing.T) {
	// Identical strings.
	if s := stringSimilarity("hello", "hello"); s != 1.0 {
		t.Errorf("expected 1.0, got %f", s)
	}
	// Completely different.
	if s := stringSimilarity("hello", "world"); s > 0.5 {
		t.Errorf("expected <0.5, got %f", s)
	}
	// Partial similarity.
	s := stringSimilarity("Southeast University", "Southeastern University")
	if s < 0.7 || s > 0.95 {
		t.Errorf("expected 0.7-0.95, got %f", s)
	}
}

func TestReviewBand(t *testing.T) {
	band := ReviewBand{Low: 0.5, High: 0.9}
	if band.Classify(0.95) != DecisionAutoMerge {
		t.Error("0.95 should auto-merge")
	}
	if band.Classify(0.3) != DecisionAutoReject {
		t.Error("0.3 should auto-reject")
	}
	if band.Classify(0.7) != DecisionHuman {
		t.Error("0.7 should go to human review")
	}
}

func TestLevenshteinMatcher(t *testing.T) {
	m := LevenshteinMatcher{
		LabelWeight: 1.0,
		AttrWeights:  map[string]float64{"email": 0.5},
		Threshold:    0.85,
	}
	c1 := EntityCandidate{Label: "Alice", Attributes: map[string]string{"email": "alice@x.com"}}
	c2 := EntityCandidate{Label: "Alice", Attributes: map[string]string{"email": "alice@x.com"}}
	score := m.Score(c1, c2)
	if score != 1.0 {
		t.Errorf("expected 1.0 for identical, got %f", score)
	}
}

func TestMerge(t *testing.T) {
	a := &graph.Node{
		ID:         "n1",
		Type:       "Person",
		Label:      "Alice",
		Attributes: map[string]string{"email": "alice@x.com"},
		Provenance: graph.Provenance{Source: "s1", ExtractedAt: time.Now()},
	}
	b := &graph.Node{
		ID:         "n2",
		Type:       "Person",
		Label:      "A. Smith",
		Attributes: map[string]string{"email": "a.smith@x.com", "phone": "555-1234"},
		Provenance: graph.Provenance{Source: "s2", ExtractedAt: time.Now()},
	}
	policy := MergePolicy{
		SourcePriority: map[string]int{"s1": 2, "s2": 1},
		KeepAlias:      true,
	}

	merged, rec := Merge(a, b, policy)
	if rec.Survivor != "n1" {
		t.Errorf("expected survivor n1, got %s", rec.Survivor)
	}
	if rec.Absorbed != "n2" {
		t.Errorf("expected absorbed n2, got %s", rec.Absorbed)
	}
	// B's label should become an alias.
	hasAlias := false
	for _, alias := range merged.Aliases {
		if alias == "A. Smith" {
			hasAlias = true
		}
	}
	if !hasAlias {
		t.Error("merged node should have b's label as alias")
	}
	// B's unique attribute should be added.
	if merged.Attributes["phone"] != "555-1234" {
		t.Error("merged node should have b's phone attribute")
	}
	// B's conflicting attribute should NOT override a's (a has higher priority).
	if merged.Attributes["email"] != "alice@x.com" {
		t.Error("a's email should win (higher priority source)")
	}
	// MergedFrom should track b.
	foundMergedFrom := false
	for _, id := range merged.MergedFrom {
		if id == "n2" {
			foundMergedFrom = true
		}
	}
	if !foundMergedFrom {
		t.Error("merged node should track n2 in MergedFrom")
	}
}
