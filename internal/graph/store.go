package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store is the in-memory knowledge graph store with JSON file persistence.
// It holds nodes, edges, events, and the ontology that governs them.
//
// Concurrency: safe for concurrent reads. Writes are serialized by a mutex.
// For large-scale graphs, swap this implementation for a SQLite/Neo4j-backed
// store — the interface stays the same.
type Store struct {
	mu       sync.RWMutex
	filePath string
	ontology *Ontology
	nodes    map[string]*Node
	edges    map[string]*Edge
	events   map[string]*Event
	// index: alias → node ID, for entity resolution (fusion).
	aliasIndex map[string]string
	// index: type → node IDs, for typed queries.
	typeIndex map[string][]string
	// mergeLog records every merge operation for reversibility.
	mergeLog []MergeRecord
}

// MergeRecord logs a single merge operation so it can be undone.
type MergeRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Survivor  string    `json:"survivor"` // node ID that survived
	absorbed  string    `json:"absorbed"` // node ID that was absorbed
	Reason    string    `json:"reason,omitempty"`
}

// NewStore creates a new graph store. If filePath is non-empty, the store
// loads from disk on construction and persists on every write.
func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath:   filePath,
		nodes:      make(map[string]*Node),
		edges:      make(map[string]*Edge),
		events:     make(map[string]*Event),
		aliasIndex: make(map[string]string),
		typeIndex:  make(map[string][]string),
	}
	if filePath != "" {
		if err := s.load(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load graph: %w", err)
		}
	}
	return s, nil
}

// SetOntology installs the governing ontology. The ontology must validate
// before installation. Existing nodes/edges are NOT re-validated — call
// ValidateAll after installing a new ontology.
func (s *Store) SetOntology(o *Ontology) error {
	if err := o.Validate(); err != nil {
		return fmt.Errorf("ontology invalid: %w", err)
	}
	s.mu.Lock()
	s.ontology = o
	s.mu.Unlock()
	return s.persist()
}

// Ontology returns the current ontology.
func (s *Store) Ontology() *Ontology {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ontology
}

// ── Node operations ─────────────────────────────────────────────

// AddNode inserts a new entity node, validated against the ontology.
// Returns an error if the node's type is not in the ontology, a required
// attribute is missing, or provenance is incomplete.
func (s *Store) AddNode(n *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ontology == nil {
		return errors.New("ontology not set — call SetOntology first")
	}
	if err := n.Validate(s.ontology); err != nil {
		return err
	}
	if _, exists := s.nodes[n.ID]; exists {
		return fmt.Errorf("node %s already exists", n.ID)
	}

	s.nodes[n.ID] = n
	s.indexNode(n)
	return s.persistLocked()
}

// GetNode retrieves a node by ID.
func (s *Store) GetNode(id string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	return n, ok
}

// FindByAlias looks up a node by label or alias (case-insensitive). Used by
// the fusion stage for entity resolution.
func (s *Store) FindByAlias(alias string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.aliasIndex[strings.ToLower(alias)]
	if !ok {
		return nil, false
	}
	n, _ := s.nodes[id]
	return n, true
}

// NodesByType returns all nodes of the given entity type (including
// subclasses).
func (s *Store) NodesByType(typeName string) []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Node
	if s.ontology != nil {
		types := append([]string{typeName}, s.ontology.SubClassesOf(typeName)...)
		typeSet := make(map[string]struct{}, len(types))
		for _, t := range types {
			typeSet[t] = struct{}{}
		}
		for _, n := range s.nodes {
			if _, ok := typeSet[n.Type]; ok {
				result = append(result, n)
			}
		}
	} else {
		for _, n := range s.nodes {
			if n.Type == typeName {
				result = append(result, n)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// AllNodes returns a snapshot of all nodes.
func (s *Store) AllNodes() []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// ── Edge operations ─────────────────────────────────────────────

// AddEdge inserts a typed relation between two nodes, validated against the
// ontology's domain/range constraints. This one validation step removes most
// hallucinated structure.
func (s *Store) AddEdge(e *Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ontology == nil {
		return errors.New("ontology not set — call SetOntology first")
	}
	subject, ok := s.nodes[e.Subject]
	if !ok {
		return fmt.Errorf("edge %s: subject node %s not found", e.ID, e.Subject)
	}
	object, ok := s.nodes[e.Object]
	if !ok {
		return fmt.Errorf("edge %s: object node %s not found", e.ID, e.Object)
	}
	if err := e.Validate(s.ontology, subject.Type, object.Type); err != nil {
		return err
	}
	if _, exists := s.edges[e.ID]; exists {
		return fmt.Errorf("edge %s already exists", e.ID)
	}

	s.edges[e.ID] = e
	return s.persistLocked()
}

// GetEdge retrieves an edge by ID.
func (s *Store) GetEdge(id string) (*Edge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.edges[id]
	return e, ok
}

// EdgesFrom returns all edges where the given node is the subject.
func (s *Store) EdgesFrom(nodeID string) []*Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Edge
	for _, e := range s.edges {
		if e.Subject == nodeID {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// EdgesTo returns all edges where the given node is the object.
func (s *Store) EdgesTo(nodeID string) []*Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Edge
	for _, e := range s.edges {
		if e.Object == nodeID {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// AllEdges returns a snapshot of all edges.
func (s *Store) AllEdges() []*Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Edge, 0, len(s.edges))
	for _, e := range s.edges {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// ── Event operations ────────────────────────────────────────────

// AddEvent inserts an event node. Events are kept separate from entity nodes.
func (s *Store) AddEvent(ev *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ev.Validate(); err != nil {
		return err
	}
	if _, exists := s.events[ev.ID]; exists {
		return fmt.Errorf("event %s already exists", ev.ID)
	}
	s.events[ev.ID] = ev
	return s.persistLocked()
}

// GetEvent retrieves an event by ID.
func (s *Store) GetEvent(id string) (*Event, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, ok := s.events[id]
	return ev, ok
}

// AllEvents returns a snapshot of all events.
func (s *Store) AllEvents() []*Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Event, 0, len(s.events))
	for _, ev := range s.events {
		result = append(result, ev)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// ── Validation ──────────────────────────────────────────────────

// ValidateAll re-checks every node and edge against the current ontology.
// Returns the first error encountered. Useful after installing a new
// ontology or before serving.
func (s *Store) ValidateAll() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ontology == nil {
		return errors.New("ontology not set")
	}
	for _, n := range s.nodes {
		if err := n.Validate(s.ontology); err != nil {
			return err
		}
	}
	for _, e := range s.edges {
		subject, ok := s.nodes[e.Subject]
		if !ok {
			return fmt.Errorf("edge %s: subject %s missing", e.ID, e.Subject)
		}
		object, ok := s.nodes[e.Object]
		if !ok {
			return fmt.Errorf("edge %s: object %s missing", e.ID, e.Object)
		}
		if err := e.Validate(s.ontology, subject.Type, object.Type); err != nil {
			return err
		}
	}
	return nil
}

// ── Stats ───────────────────────────────────────────────────────

// Stats holds summary counts for the graph store.
type Stats struct {
	NodeCount  int            `json:"node_count"`
	EdgeCount  int            `json:"edge_count"`
	EventCount int            `json:"event_count"`
	TypeCounts map[string]int `json:"type_counts"`
	RelCounts  map[string]int `json:"relation_counts"`
	MergeCount int            `json:"merge_count"`
}

// Stats returns summary statistics about the current graph.
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{
		NodeCount:  len(s.nodes),
		EdgeCount:  len(s.edges),
		EventCount: len(s.events),
		MergeCount: len(s.mergeLog),
		TypeCounts: make(map[string]int),
		RelCounts:  make(map[string]int),
	}
	for _, n := range s.nodes {
		st.TypeCounts[n.Type]++
	}
	for _, e := range s.edges {
		st.RelCounts[e.Relation]++
	}
	return st
}

// ── Internal helpers ────────────────────────────────────────────

func (s *Store) indexNode(n *Node) {
	s.aliasIndex[strings.ToLower(n.Label)] = n.ID
	for _, a := range n.Aliases {
		s.aliasIndex[strings.ToLower(a)] = n.ID
	}
	s.typeIndex[n.Type] = append(s.typeIndex[n.Type], n.ID)
}

func (s *Store) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	return s.persistLockedPath(s.filePath)
}

func (s *Store) persist() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistLocked()
}

func (s *Store) persistLockedPath(path string) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create graph dir: %w", err)
		}
	}
	data := struct {
		Ontology *Ontology     `json:"ontology,omitempty"`
		Nodes    []*Node       `json:"nodes"`
		Edges    []*Edge       `json:"edges"`
		Events   []*Event      `json:"events,omitempty"`
		MergeLog []MergeRecord `json:"merge_log,omitempty"`
	}{
		Ontology: s.ontology,
		Nodes:    s.allNodesLocked(),
		Edges:    s.allEdgesLocked(),
		Events:   s.allEventsLocked(),
		MergeLog: s.mergeLog,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	return os.WriteFile(path, b, 0o644)
}

func (s *Store) load() error {
	if s.filePath == "" {
		return nil
	}
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var data struct {
		Ontology *Ontology     `json:"ontology,omitempty"`
		Nodes    []*Node       `json:"nodes"`
		Edges    []*Edge       `json:"edges"`
		Events   []*Event      `json:"events,omitempty"`
		MergeLog []MergeRecord `json:"merge_log,omitempty"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("unmarshal graph: %w", err)
	}
	s.ontology = data.Ontology
	for _, n := range data.Nodes {
		s.nodes[n.ID] = n
		s.indexNode(n)
	}
	for _, e := range data.Edges {
		s.edges[e.ID] = e
	}
	for _, ev := range data.Events {
		s.events[ev.ID] = ev
	}
	s.mergeLog = data.MergeLog
	return nil
}

func (s *Store) allNodesLocked() []*Node {
	result := make([]*Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *Store) allEdgesLocked() []*Edge {
	result := make([]*Edge, 0, len(s.edges))
	for _, e := range s.edges {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *Store) allEventsLocked() []*Event {
	result := make([]*Event, 0, len(s.events))
	for _, ev := range s.events {
		result = append(result, ev)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
