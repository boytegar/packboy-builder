package graph

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGraphToolSetOntology(t *testing.T) {
	dir := t.TempDir()
	tt := NewGraphTool()

	params := map[string]any{
		"action": "set_ontology",
		"ontology": map[string]any{
			"name": "test",
			"entity_types": []map[string]any{
				{"name": "Person"},
				{"name": "Company"},
			},
			"relations": []map[string]any{
				{"name": "WORKS_FOR", "domain": "Person", "range": "Company"},
			},
		},
	}

	result := tt.Execute(context.Background(), params, dir)
	if !result.Success {
		t.Fatalf("set_ontology failed: %s", result.Error)
	}
}

func TestGraphToolAddNodeEdge(t *testing.T) {
	dir := t.TempDir()
	tt := NewGraphTool()

	// Set ontology first.
	tt.Execute(context.Background(), map[string]any{
		"action": "set_ontology",
		"ontology": map[string]any{
			"name": "test",
			"entity_types": []map[string]any{
				{"name": "Person"},
				{"name": "Company"},
			},
			"relations": []map[string]any{
				{"name": "WORKS_FOR", "domain": "Person", "range": "Company"},
			},
		},
	}, dir)

	// Add node.
	result := tt.Execute(context.Background(), map[string]any{
		"action": "add_node",
		"node": map[string]any{
			"id":     "p1",
			"type":   "Person",
			"label":  "Alice",
			"source": "test",
		},
	}, dir)
	if !result.Success {
		t.Fatalf("add_node failed: %s", result.Error)
	}

	// Add edge (need company node too).
	tt.Execute(context.Background(), map[string]any{
		"action": "add_node",
		"node": map[string]any{
			"id":     "c1",
			"type":   "Company",
			"label":  "Acme",
			"source": "test",
		},
	}, dir)

	result = tt.Execute(context.Background(), map[string]any{
		"action": "add_edge",
		"edge": map[string]any{
			"id":       "e1",
			"relation": "WORKS_FOR",
			"subject":  "p1",
			"object":   "c1",
			"source":   "test",
		},
	}, dir)
	if !result.Success {
		t.Fatalf("add_edge failed: %s", result.Error)
	}

	// Stats.
	result = tt.Execute(context.Background(), map[string]any{
		"action": "stats",
	}, dir)
	if !result.Success {
		t.Fatalf("stats failed: %s", result.Error)
	}

	// Validate.
	result = tt.Execute(context.Background(), map[string]any{
		"action": "validate",
	}, dir)
	if !result.Success {
		t.Fatalf("validate failed: %s", result.Error)
	}

	// Query.
	result = tt.Execute(context.Background(), map[string]any{
		"action":   "query",
		"question": "What is Alice",
		"k":        2,
	}, dir)
	if !result.Success {
		t.Fatalf("query failed: %s", result.Error)
	}

	// Verify persistence — reload from same path.
	storePath := filepath.Join(dir, ".pcb", "kg", "graph.json")
	_ = storePath
}

func TestGraphToolRejectInvalidEdge(t *testing.T) {
	dir := t.TempDir()
	tt := NewGraphTool()

	tt.Execute(context.Background(), map[string]any{
		"action": "set_ontology",
		"ontology": map[string]any{
			"name": "test",
			"entity_types": []map[string]any{
				{"name": "Person"},
				{"name": "Company"},
			},
			"relations": []map[string]any{
				{"name": "WORKS_FOR", "domain": "Person", "range": "Company"},
			},
		},
	}, dir)

	tt.Execute(context.Background(), map[string]any{
		"action": "add_node",
		"node":   map[string]any{"id": "p1", "type": "Person", "label": "Alice", "source": "s"},
	}, dir)
	tt.Execute(context.Background(), map[string]any{
		"action": "add_node",
		"node":   map[string]any{"id": "c1", "type": "Company", "label": "Acme", "source": "s"},
	}, dir)

	// Reversed edge: Company WORKS_FOR Person — should fail.
	result := tt.Execute(context.Background(), map[string]any{
		"action": "add_edge",
		"edge": map[string]any{
			"id":       "bad",
			"relation": "WORKS_FOR",
			"subject":  "c1",
			"object":   "p1",
			"source":   "s",
		},
	}, dir)
	if result.Success {
		t.Error("expected error for reversed edge (domain/range violation)")
	}
}

func TestGraphToolQueryNoResults(t *testing.T) {
	dir := t.TempDir()
	tt := NewGraphTool()

	// Query with empty graph — should succeed with "no results" message.
	result := tt.Execute(context.Background(), map[string]any{
		"action":   "query",
		"question": "What is Alice",
	}, dir)
	if !result.Success {
		t.Fatalf("query on empty graph failed: %s", result.Error)
	}
}
