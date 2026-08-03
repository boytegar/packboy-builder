package graph

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ── Provenance ──────────────────────────────────────────────────

// Provenance records where a fact came from. Every node and edge in the
// knowledge graph carries provenance — non-negotiable. A triple with no
// evidence span is a hallucination with extra steps.
type Provenance struct {
	// Source is the URI or path of the origin document (file, URL, table row).
	Source string `json:"source"`

	// Span is the verbatim evidence text that justifies the fact. May be empty
	// for structured sources (table rows), but unstructured extraction must
	// always populate it.
	Span string `json:"span,omitempty"`

	// ExtractedAt is when the extraction happened.
	ExtractedAt time.Time `json:"extracted_at"`

	// Confidence is the extractor's self-reported confidence (0.0–1.0).
	Confidence float64 `json:"confidence,omitempty"`

	// Extractor identifies the method or model that produced the fact
	// (e.g. "llm:gpt-4o", "rule:dict-v2", "manual").
	Extractor string `json:"extractor,omitempty"`
}

// Validate checks that provenance has the minimum required fields.
func (p Provenance) Validate() error {
	if p.Source == "" {
		return errors.New("provenance: source is required")
	}
	if p.ExtractedAt.IsZero() {
		return errors.New("provenance: extracted_at is required")
	}
	return nil
}

// ── Node (Entity) ───────────────────────────────────────────────

// Node is an entity in the knowledge graph. It carries an entity type from
// the ontology, a set of attributes, and provenance.
type Node struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`  // entity type from ontology
	Label      string            `json:"label"` // human-readable name
	Attributes map[string]string `json:"attributes,omitempty"`
	Aliases    []string          `json:"aliases,omitempty"` // surface forms that resolved to this node
	Provenance Provenance        `json:"provenance"`
	// MergedFrom tracks node IDs that were merged into this one. Enables
	// reversible merges — undo by restoring the merged nodes.
	MergedFrom []string `json:"merged_from,omitempty"`
}

// Validate checks that a node is well-formed against the given ontology.
func (n *Node) Validate(o *Ontology) error {
	if n.ID == "" {
		return errors.New("node: id is required")
	}
	if n.Type == "" {
		return errors.New("node: type is required")
	}
	et := o.EntityType(n.Type)
	if et == nil {
		// Check subclass — the type may be a subclass of a declared type.
		found := false
		for _, e := range o.EntityTypes {
			if e.Name == n.Type {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("node %s: type %s not in ontology", n.ID, n.Type)
		}
	}
	if n.Label == "" {
		return fmt.Errorf("node %s: label is required", n.ID)
	}
	if err := n.Provenance.Validate(); err != nil {
		return fmt.Errorf("node %s: %w", n.ID, err)
	}

	// Check required attributes.
	if et != nil {
		for _, attr := range et.Attributes {
			if attr.Required {
				if _, ok := n.Attributes[attr.Name]; !ok {
					return fmt.Errorf("node %s: required attribute %s missing", n.ID, attr.Name)
				}
			}
		}
	}
	return nil
}

// HasAlias returns true if the node has the given alias (case-insensitive).
func (n *Node) HasAlias(alias string) bool {
	if strings.EqualFold(n.Label, alias) {
		return true
	}
	for _, a := range n.Aliases {
		if strings.EqualFold(a, alias) {
			return true
		}
	}
	return false
}

// ── Edge (Relation) ─────────────────────────────────────────────

// Edge is a typed relation between two nodes, constrained by the ontology's
// domain/range. Every edge carries provenance.
type Edge struct {
	ID         string            `json:"id"`
	Relation   string            `json:"relation"` // relation name from ontology
	Subject    string            `json:"subject"`  // node ID
	Object     string            `json:"object"`   // node ID
	Attributes map[string]string `json:"attributes,omitempty"`
	Provenance Provenance        `json:"provenance"`
}

// Validate checks that an edge is well-formed and domain/range-valid against
// the ontology. This one validation step removes most hallucinated structure.
func (e *Edge) Validate(o *Ontology, subjectType, objectType string) error {
	if e.ID == "" {
		return errors.New("edge: id is required")
	}
	if e.Relation == "" {
		return errors.New("edge: relation is required")
	}
	if e.Subject == "" || e.Object == "" {
		return errors.New("edge: subject and object are required")
	}
	r := o.Relation(e.Relation)
	if r == nil {
		return fmt.Errorf("edge %s: relation %s not in ontology", e.ID, e.Relation)
	}
	if !o.RelationValidFor(e.Relation, subjectType, objectType) {
		return fmt.Errorf("edge %s: relation %s requires %s→%s, got %s→%s",
			e.ID, e.Relation, r.Domain, r.Range, subjectType, objectType)
	}
	if err := e.Provenance.Validate(); err != nil {
		return fmt.Errorf("edge %s: %w", e.ID, err)
	}
	return nil
}

// ── Event ───────────────────────────────────────────────────────

// Event is a first-class node for things that happened (not things that are).
// Events have a trigger, typed arguments, and a time anchor. Event nodes are
// kept separate from entity nodes — never collapse a cause into an attribute.
type Event struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`           // event type
	Trigger    string            `json:"trigger"`        // the word/phrase that signals the event
	Time       time.Time         `json:"time,omitempty"` // time anchor
	Arguments  []EventArgument   `json:"arguments,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Provenance Provenance        `json:"provenance"`
}

// EventArgument is a typed role-filled participant in an event.
type EventArgument struct {
	Role     string `json:"role"`    // e.g. "agent", "patient", "instrument"
	NodeID   string `json:"node_id"` // entity node filling the role
	NodeType string `json:"node_type,omitempty"`
}

// Validate checks that an event is well-formed.
func (ev *Event) Validate() error {
	if ev.ID == "" {
		return errors.New("event: id is required")
	}
	if ev.Type == "" {
		return errors.New("event: type is required")
	}
	if ev.Trigger == "" {
		return fmt.Errorf("event %s: trigger is required", ev.ID)
	}
	if err := ev.Provenance.Validate(); err != nil {
		return fmt.Errorf("event %s: %w", ev.ID, err)
	}
	return nil
}
