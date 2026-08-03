package graph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOntologyValidate(t *testing.T) {
	o := &Ontology{
		Name: "test",
		EntityTypes: []EntityType{
			{Name: "Person", Identifies: "email"},
			{Name: "Company", Identifies: "domain"},
			{Name: "Employee", SubClassOf: "Person"},
		},
		Relations: []Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company", Cardinality: "N:1"},
			{Name: "ACQUIRED", Domain: "Company", Range: "Company", Cardinality: "N:1"},
		},
	}

	if err := o.Validate(); err != nil {
		t.Fatalf("valid ontology failed: %v", err)
	}

	// Test relation lookup.
	r := o.Relation("WORKS_FOR")
	if r == nil {
		t.Fatal("WORKS_FOR relation not found")
	}
	if r.Domain != "Person" || r.Range != "Company" {
		t.Fatalf("unexpected domain/range: %s/%s", r.Domain, r.Range)
	}

	// Test RelationValidFor.
	if !o.RelationValidFor("WORKS_FOR", "Person", "Company") {
		t.Error("RelationValidFor should accept Person→Company")
	}
	if o.RelationValidFor("WORKS_FOR", "Company", "Person") {
		t.Error("RelationValidFor should reject Company→Person")
	}

	// Test subclass substitution.
	if !o.RelationValidFor("WORKS_FOR", "Employee", "Company") {
		t.Error("RelationValidFor should accept Employee→Company (subclass of Person)")
	}

	// Test IsA (subclass).
	if !o.IsA("Employee", "Person") {
		t.Error("IsA should recognize Employee as a Person")
	}
	if o.IsA("Company", "Person") {
		t.Error("IsA should reject Company as a Person")
	}
}

func TestOntologyValidateErrors(t *testing.T) {
	// Missing name.
	o := &Ontology{EntityTypes: []EntityType{{Name: "X"}}}
	if err := o.Validate(); err == nil {
		t.Error("expected error for missing name")
	}

	// Missing entity types.
	o = &Ontology{Name: "test"}
	if err := o.Validate(); err == nil {
		t.Error("expected error for missing entity types")
	}

	// Relation with undeclared domain.
	o = &Ontology{
		Name:        "test",
		EntityTypes: []EntityType{{Name: "Person"}},
		Relations:   []Relation{{Name: "KNOWS", Domain: "Person", Range: "Ghost"}},
	}
	if err := o.Validate(); err == nil {
		t.Error("expected error for undeclared range type")
	}
}

func TestNodeValidate(t *testing.T) {
	o := &Ontology{
		Name: "test",
		EntityTypes: []EntityType{
			{Name: "Person", Attributes: []Attribute{{Name: "email", Required: true}}},
		},
	}

	// Valid node.
	n := &Node{
		ID:         "p1",
		Type:       "Person",
		Label:      "Alice",
		Attributes: map[string]string{"email": "alice@example.com"},
		Provenance: Provenance{Source: "test", ExtractedAt: time.Now()},
	}
	if err := n.Validate(o); err != nil {
		t.Fatalf("valid node failed: %v", err)
	}

	// Missing required attribute.
	n2 := &Node{
		ID:         "p2",
		Type:       "Person",
		Label:      "Bob",
		Provenance: Provenance{Source: "test", ExtractedAt: time.Now()},
	}
	if err := n2.Validate(o); err == nil {
		t.Error("expected error for missing required attribute")
	}

	// Missing provenance.
	n3 := &Node{
		ID:         "p3",
		Type:       "Person",
		Label:      "Carol",
		Attributes: map[string]string{"email": "c@x.com"},
	}
	if err := n3.Validate(o); err == nil {
		t.Error("expected error for missing provenance")
	}
}

func TestEdgeValidate(t *testing.T) {
	o := &Ontology{
		Name: "test",
		EntityTypes: []EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
		Relations: []Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company"},
		},
	}

	// Valid edge.
	e := &Edge{
		ID:         "e1",
		Relation:   "WORKS_FOR",
		Subject:    "p1",
		Object:     "c1",
		Provenance: Provenance{Source: "test", ExtractedAt: time.Now()},
	}
	if err := e.Validate(o, "Person", "Company"); err != nil {
		t.Fatalf("valid edge failed: %v", err)
	}

	// Domain/range violation.
	if err := e.Validate(o, "Company", "Person"); err == nil {
		t.Error("expected domain/range error")
	}
}

func TestStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Set ontology.
	o := &Ontology{
		Name: "test",
		EntityTypes: []EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
		Relations: []Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company"},
		},
	}
	if err := s.SetOntology(o); err != nil {
		t.Fatalf("SetOntology: %v", err)
	}

	// Add nodes.
	p1 := &Node{ID: "p1", Type: "Person", Label: "Alice", Provenance: Provenance{Source: "s", ExtractedAt: time.Now()}}
	c1 := &Node{ID: "c1", Type: "Company", Label: "Acme", Provenance: Provenance{Source: "s", ExtractedAt: time.Now()}}
	if err := s.AddNode(p1); err != nil {
		t.Fatalf("AddNode p1: %v", err)
	}
	if err := s.AddNode(c1); err != nil {
		t.Fatalf("AddNode c1: %v", err)
	}

	// Find by alias.
	found, ok := s.FindByAlias("Alice")
	if !ok {
		t.Fatal("FindByAlias failed")
	}
	if found.ID != "p1" {
		t.Fatalf("wrong node: %s", found.ID)
	}

	// Add edge.
	e1 := &Edge{ID: "e1", Relation: "WORKS_FOR", Subject: "p1", Object: "c1", Provenance: Provenance{Source: "s", ExtractedAt: time.Now()}}
	if err := s.AddEdge(e1); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	// Query edges.
	from := s.EdgesFrom("p1")
	if len(from) != 1 {
		t.Fatalf("expected 1 edge from p1, got %d", len(from))
	}
	to := s.EdgesTo("c1")
	if len(to) != 1 {
		t.Fatalf("expected 1 edge to c1, got %d", len(to))
	}

	// Stats.
	st := s.Stats()
	if st.NodeCount != 2 {
		t.Fatalf("expected 2 nodes, got %d", st.NodeCount)
	}
	if st.EdgeCount != 1 {
		t.Fatalf("expected 1 edge, got %d", st.EdgeCount)
	}

	// Reload from disk.
	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload NewStore: %v", err)
	}
	st2 := s2.Stats()
	if st2.NodeCount != 2 {
		t.Fatalf("after reload: expected 2 nodes, got %d", st2.NodeCount)
	}

	_ = os.RemoveAll(path)
}

func TestStoreRejectInvalidEdge(t *testing.T) {
	s, _ := NewStore("")
	o := &Ontology{
		Name: "test",
		EntityTypes: []EntityType{
			{Name: "Person"},
			{Name: "Company"},
		},
		Relations: []Relation{
			{Name: "WORKS_FOR", Domain: "Person", Range: "Company"},
		},
	}
	s.SetOntology(o)

	p1 := &Node{ID: "p1", Type: "Person", Label: "Alice", Provenance: Provenance{Source: "s", ExtractedAt: time.Now()}}
	c1 := &Node{ID: "c1", Type: "Company", Label: "Acme", Provenance: Provenance{Source: "s", ExtractedAt: time.Now()}}
	s.AddNode(p1)
	s.AddNode(c1)

	// Invalid: Company WORKS_FOR Person (wrong direction).
	badEdge := &Edge{ID: "bad", Relation: "WORKS_FOR", Subject: "c1", Object: "p1", Provenance: Provenance{Source: "s", ExtractedAt: time.Now()}}
	if err := s.AddEdge(badEdge); err == nil {
		t.Error("expected domain/range error for reversed edge")
	}
}
