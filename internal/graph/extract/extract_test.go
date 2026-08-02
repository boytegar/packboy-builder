package extract

import (
	"testing"

	"github.com/boytegar/packboy-builder/internal/graph"
)

func TestRouterClassify(t *testing.T) {
	o := &graph.Ontology{
		Name: "test",
		EntityTypes: []graph.EntityType{{Name: "Person"}},
	}
	r := NewRouter(o)

	tests := []struct {
		src    Source
		want   SourceKind
	}{
		{Source{Format: "csv"}, SourceStructured},
		{Source{Format: "json"}, SourceStructured},
		{Source{Format: "html"}, SourceSemiStructured},
		{Source{Format: "xml"}, SourceSemiStructured},
		{Source{Format: "pdf"}, SourceUnstructured},
		{Source{Format: "txt"}, SourceUnstructured},
		{Source{Kind: SourceStructured, Format: "csv"}, SourceStructured}, // explicit kind
	}
	for _, tt := range tests {
		got := r.Classify(tt.src)
		if got != tt.want {
			t.Errorf("Classify(%+v) = %s, want %s", tt.src, got, tt.want)
		}
	}
}

func TestValidateEntities(t *testing.T) {
	o := &graph.Ontology{
		Name: "test",
		EntityTypes: []graph.EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
	}
	r := NewRouter(o)

	candidates := []EntityCandidate{
		{Type: "Person", Label: "Alice", Span: "Alice works here"},
		{Type: "Ghost", Label: "Casper", Span: "Casper is a ghost"},
		{Type: "Company", Label: "Acme", Span: "Acme Inc"},
	}
	valid, errs := r.ValidateEntities(candidates)
	if len(valid) != 2 {
		t.Errorf("expected 2 valid, got %d", len(valid))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateRelations(t *testing.T) {
	o := &graph.Ontology{
		Name: "test",
		EntityTypes: []graph.EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
		Relations: []graph.Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company"},
		},
	}
	r := NewRouter(o)

	resolved := map[string]string{
		"Alice": "Person",
		"Acme":  "Company",
		"Casper": "Ghost", // not in ontology
	}

	candidates := []RelationCandidate{
		{Relation: "WORKS_FOR", SubjectLabel: "Alice", ObjectLabel: "Acme", Span: "Alice works for Acme"},
		{Relation: "WORKS_FOR", SubjectLabel: "Acme", ObjectLabel: "Alice", Span: "wrong direction"},
		{Relation: "KNOWS", SubjectLabel: "Alice", ObjectLabel: "Acme", Span: "unknown relation"},
		{Relation: "WORKS_FOR", SubjectLabel: "Alice", ObjectLabel: "Casper", Span: "bad type"},
	}
	valid, errs := r.ValidateRelations(candidates, resolved)
	if len(valid) != 1 {
		t.Errorf("expected 1 valid, got %d", len(valid))
	}
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errs))
	}
}

func TestRejectionRules(t *testing.T) {
	candidates := []RelationCandidate{
		{Relation: "R", SubjectLabel: "A", ObjectLabel: "B", Span: "evidence", Confidence: 0.9},
		{Relation: "R", SubjectLabel: "A", ObjectLabel: "C", Span: "", Confidence: 0.9},           // empty span
		{Relation: "R", SubjectLabel: "D", ObjectLabel: "E", Span: "evidence", Confidence: 0.3},  // low confidence
	}
	rules := []RejectionRule{
		RejectLowConfidence(0.5),
		RejectEmptySpan(),
	}
	kept, rejected := ApplyRejectionRules(candidates, rules)
	if len(kept) != 1 {
		t.Errorf("expected 1 kept, got %d", len(kept))
	}
	if rejected != 2 {
		t.Errorf("expected 2 rejected, got %d", rejected)
	}
}

func TestCommonFailureModes(t *testing.T) {
	for _, kind := range []SourceKind{SourceStructured, SourceSemiStructured, SourceUnstructured} {
		modes := CommonFailureModes(kind)
		if len(modes) != 5 {
			t.Errorf("kind %s: expected 5 modes, got %d", kind, len(modes))
		}
	}
}
