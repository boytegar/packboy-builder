package serve

import (
	"testing"
	"time"

	"github.com/boytegar/packboy-builder/internal/graph"
)

func TestClassifyQuestion(t *testing.T) {
	tests := []struct {
		q    string
		want QuestionType
	}{
		{"What is the capital of France?", QTLookup},
		{"Who worked with Alice on Project X?", QTMultiHop},
		{"Which company acquired Acme Corp?", QTMultiHop},
		{"Define ontology", QTLookup},
		{"Everything related to Bob", QTSubgraph},
		{"Summarize the article", QTVectorOnly},
	}
	for _, tt := range tests {
		got := ClassifyQuestion(tt.q)
		if got != tt.want {
			t.Errorf("ClassifyQuestion(%q) = %s, want %s", tt.q, got, tt.want)
		}
	}
}

func TestRetrieverEntityLookup(t *testing.T) {
	s, _ := graph.NewStore("")
	s.SetOntology(&graph.Ontology{
		Name: "test",
		EntityTypes: []graph.EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
		Relations: []graph.Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company"},
		},
	})

	p1 := &graph.Node{ID: "p1", Type: "Person", Label: "Alice", Provenance: graph.Provenance{Source: "s", ExtractedAt: time.Now()}}
	s.AddNode(p1)

	r := NewRetriever(s)
	// "What is Alice" should find Alice.
	ctx, qt := r.Retrieve("What is Alice", 2)
	if qt != QTLookup {
		t.Errorf("expected QTLookup, got %s", qt)
	}
	if ctx == "" {
		t.Error("expected non-empty context for entity lookup")
	}
}

func TestRetrieverKHop(t *testing.T) {
	s, _ := graph.NewStore("")
	s.SetOntology(&graph.Ontology{
		Name: "test",
		EntityTypes: []graph.EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
		Relations: []graph.Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company"},
		},
	})

	s.AddNode(&graph.Node{ID: "p1", Type: "Person", Label: "Alice", Provenance: graph.Provenance{Source: "s", ExtractedAt: time.Now()}})
	s.AddNode(&graph.Node{ID: "c1", Type: "Company", Label: "Acme", Provenance: graph.Provenance{Source: "s", ExtractedAt: time.Now()}})
	s.AddEdge(&graph.Edge{ID: "e1", Relation: "WORKS_FOR", Subject: "p1", Object: "c1", Provenance: graph.Provenance{Source: "s", ExtractedAt: time.Now()}})

	r := NewRetriever(s)
	ctx, qt := r.Retrieve("Who with Acme", 2)
	if qt != QTMultiHop {
		t.Errorf("expected QTMultiHop, got %s", qt)
	}
	if ctx == "" {
		t.Error("expected non-empty context for k-hop traversal")
	}
}

func TestEvalSetScore(t *testing.T) {
	items := []EvalItem{
		{ID: "q1", Question: "Q1", Answer: "Alice", Type: QTLookup},
		{ID: "q2", Question: "Q2", Answer: "Acme", Type: QTLookup},
		{ID: "q3", Question: "Q3", Answer: "Bob", Type: QTMultiHop},
	}
	es := NewEvalSet(items, "exact_match")

	// All correct.
	correct, total := es.Score(map[string]string{"q1": "Alice", "q2": "Acme", "q3": "Bob"})
	if correct != 3 || total != 3 {
		t.Fatalf("expected 3/3, got %d/%d", correct, total)
	}

	// One wrong.
	correct, total = es.Score(map[string]string{"q1": "Alice", "q2": "Wrong", "q3": "Bob"})
	if correct != 2 || total != 3 {
		t.Fatalf("expected 2/3, got %d/%d", correct, total)
	}
}

func TestEvalSetCompare(t *testing.T) {
	items := []EvalItem{
		{ID: "q1", Answer: "Alice"},
		{ID: "q2", Answer: "Acme"},
	}
	es := NewEvalSet(items, "exact_match")

	// Graph wins 2/2, vector wins 1/2.
	graphAns := map[string]string{"q1": "Alice", "q2": "Acme"}
	vectorAns := map[string]string{"q1": "Alice", "q2": "Wrong"}
	if !es.Compare(graphAns, vectorAns) {
		t.Error("graph should win with 2/2 vs 1/2")
	}
}

func TestSerializeSubgraph(t *testing.T) {
	nodes := []*graph.Node{
		{ID: "p1", Type: "Person", Label: "Alice", Attributes: map[string]string{"email": "a@x.com"}},
		{ID: "c1", Type: "Company", Label: "Acme"},
	}
	edges := []*graph.Edge{
		{ID: "e1", Relation: "WORKS_FOR", Subject: "p1", Object: "c1"},
	}
	out := SerializeSubgraph(nodes, edges, 500)
	if out == "" {
		t.Error("expected non-empty serialization")
	}
}
