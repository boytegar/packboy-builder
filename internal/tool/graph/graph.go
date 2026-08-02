package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boytegar/packboy-builder/internal/core"
	kgraph "github.com/boytegar/packboy-builder/internal/graph"
	"github.com/boytegar/packboy-builder/internal/graph/serve"
	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

const IconGraph = "🔗"

// GraphTool exposes knowledge-graph operations to the agent: set ontology,
// add entities/relations, query via GraphRAG, and inspect stats. The graph
// persists to a JSON file in the project's .pcb/kg/ directory.
type GraphTool struct{}

// NewGraphTool creates a graph tool. The store path is resolved per-invocation
// from the agent's cwd, so the tool is stateless and safe to register once.
func NewGraphTool() *GraphTool {
	return &GraphTool{}
}

func (t *GraphTool) Name() string { return "Graph" }

func (t *GraphTool) Description() string {
	return `Knowledge graph operations — the "what agents remember" half of graph engineering.

Actions:
- "set_ontology": install the governing schema (entity types + relations with domain/range). Must run before any node/edge operations.
- "add_node": add an entity (requires id, type, label, source, extracted_at). Provenance is non-negotiable.
- "add_edge": add a typed relation (requires id, relation, subject, object, source). Domain/range validated against ontology.
- "add_event": add an event node (trigger + time + arguments).
- "query": retrieve by question (entity lookup / k-hop traversal / subgraph).
- "stats": summary counts of nodes, edges, events, types.
- "validate": re-check all nodes/edges against the current ontology.

Schema first, always. Provenance on every fact. Fuse before storing.`
}

func (t *GraphTool) Icon() string { return IconGraph }

func (t *GraphTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name:        "Graph",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{"set_ontology", "add_node", "add_edge", "add_event", "query", "stats", "validate"},
					"description": "The graph operation to perform.",
				},
				"ontology": map[string]any{
					"type": "object",
					"description": "Ontology definition (for set_ontology). {name, entity_types:[{name,description?,sub_class_of?,identifies?,attributes:[{name,type,required?}]}], relations:[{name,domain,range,cardinality?,inverse?}]}",
				},
				"node": map[string]any{
					"type": "object",
					"description": "Entity node (for add_node). {id,type,label,attributes?,aliases?,source,extracted_at,confidence?}",
				},
				"edge": map[string]any{
					"type": "object",
					"description": "Relation edge (for add_edge). {id,relation,subject,object,source,extracted_at,confidence?}",
				},
				"event": map[string]any{
					"type": "object",
					"description": "Event node (for add_event). {id,type,trigger,time?,arguments?,source,extracted_at}",
				},
				"question": map[string]any{
					"type": "string",
					"description": "Natural-language question (for query). Routed to entity lookup, k-hop traversal, or subgraph extraction.",
				},
				"k": map[string]any{
					"type": "integer",
					"description": "Hop depth for k-hop traversal (default 2).",
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t *GraphTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	action := tool.GetString(params, "action")
	if action == "" {
		return toolresult.NewErrorResult(t.Name(), "action is required")
	}

	storePath := filepath.Join(cwd, ".pcb", "kg", "graph.json")
	store, err := kgraph.NewStore(storePath)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("failed to open graph store: %v", err))
	}

	switch action {
	case "set_ontology":
		return t.setOntology(store, params)
	case "add_node":
		return t.addNode(store, params)
	case "add_edge":
		return t.addEdge(store, params)
	case "add_event":
		return t.addEvent(store, params)
	case "query":
		return t.query(store, params)
	case "stats":
		return t.stats(store)
	case "validate":
		return t.validate(store)
	default:
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *GraphTool) setOntology(store *kgraph.Store, params map[string]any) toolresult.ToolResult {
	ontoRaw, ok := params["ontology"]
	if !ok {
		return toolresult.NewErrorResult(t.Name(), "ontology is required for set_ontology")
	}
	data, err := json.Marshal(ontoRaw)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("marshal ontology: %v", err))
	}
	var o kgraph.Ontology
	if err := json.Unmarshal(data, &o); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("unmarshal ontology: %v", err))
	}
	if err := store.SetOntology(&o); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("set ontology: %v", err))
	}
	return t.success(o.Summary())
}

func (t *GraphTool) addNode(store *kgraph.Store, params map[string]any) toolresult.ToolResult {
	nodeRaw, ok := params["node"]
	if !ok {
		return toolresult.NewErrorResult(t.Name(), "node is required for add_node")
	}
	n, err := parseNode(nodeRaw)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}
	if err := store.AddNode(n); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("add node: %v", err))
	}
	return t.success(fmt.Sprintf("✓ node %s added (%s: %s)", n.ID, n.Type, n.Label))
}

func (t *GraphTool) addEdge(store *kgraph.Store, params map[string]any) toolresult.ToolResult {
	edgeRaw, ok := params["edge"]
	if !ok {
		return toolresult.NewErrorResult(t.Name(), "edge is required for add_edge")
	}
	e, err := parseEdge(edgeRaw)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}
	if err := store.AddEdge(e); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("add edge: %v", err))
	}
	return t.success(fmt.Sprintf("✓ edge %s added (%s --%s--> %s)", e.ID, e.Subject, e.Relation, e.Object))
}

func (t *GraphTool) addEvent(store *kgraph.Store, params map[string]any) toolresult.ToolResult {
	eventRaw, ok := params["event"]
	if !ok {
		return toolresult.NewErrorResult(t.Name(), "event is required for add_event")
	}
	ev, err := parseEvent(eventRaw)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}
	if err := store.AddEvent(ev); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("add event: %v", err))
	}
	return t.success(fmt.Sprintf("✓ event %s added (type=%s, trigger=%s)", ev.ID, ev.Type, ev.Trigger))
}

func (t *GraphTool) query(store *kgraph.Store, params map[string]any) toolresult.ToolResult {
	question := tool.GetString(params, "question")
	if question == "" {
		return toolresult.NewErrorResult(t.Name(), "question is required for query")
	}
	k := tool.GetInt(params, "k", 2)

	r := serve.NewRetriever(store)
	ctx, qt := r.Retrieve(question, k)
	if ctx == "" {
		return t.success(fmt.Sprintf("no results for %q (strategy: %s)", question, qt))
	}
	return t.success(fmt.Sprintf("strategy: %s\n\n%s", qt, ctx))
}

func (t *GraphTool) stats(store *kgraph.Store) toolresult.ToolResult {
	st := store.Stats()
	b, _ := json.MarshalIndent(st, "", "  ")
	return t.success(string(b))
}

func (t *GraphTool) validate(store *kgraph.Store) toolresult.ToolResult {
	if err := store.ValidateAll(); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("validation failed: %v", err))
	}
	return t.success("✓ all nodes and edges valid against ontology")
}

// success is a shorthand for building a success ToolResult.
func (t *GraphTool) success(output string) toolresult.ToolResult {
	return toolresult.ToolResult{
		Success: true,
		Output:  output,
		Metadata: toolresult.ResultMetadata{
			Title: t.Name(),
			Icon:  t.Icon(),
		},
	}
}

// ── Parsing helpers ────────────────────────────────────────────

func parseNode(raw any) (*kgraph.Node, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal node: %w", err)
	}
	// Use a flat struct that accepts top-level source/confidence and maps them
	// into Provenance. This matches how agents call the tool.
	var flat struct {
		ID         string            `json:"id"`
		Type       string            `json:"type"`
		Label      string            `json:"label"`
		Attributes map[string]string `json:"attributes,omitempty"`
		Aliases    []string          `json:"aliases,omitempty"`
		Source     string            `json:"source"`
		ExtractedAt time.Time       `json:"extracted_at,omitempty"`
		Confidence float64          `json:"confidence,omitempty"`
		Extractor  string           `json:"extractor,omitempty"`
	}
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, fmt.Errorf("unmarshal node: %w", err)
	}
	n := &kgraph.Node{
		ID:         flat.ID,
		Type:       flat.Type,
		Label:      flat.Label,
		Attributes: flat.Attributes,
		Aliases:    flat.Aliases,
		Provenance: kgraph.Provenance{
			Source:      flat.Source,
			ExtractedAt: flat.ExtractedAt,
			Confidence:  flat.Confidence,
			Extractor:   flat.Extractor,
		},
	}
	if n.Provenance.ExtractedAt.IsZero() {
		n.Provenance.ExtractedAt = time.Now()
	}
	return n, nil
}

func parseEdge(raw any) (*kgraph.Edge, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal edge: %w", err)
	}
	var flat struct {
		ID         string            `json:"id"`
		Relation   string            `json:"relation"`
		Subject    string            `json:"subject"`
		Object     string            `json:"object"`
		Attributes map[string]string `json:"attributes,omitempty"`
		Source     string            `json:"source"`
		ExtractedAt time.Time       `json:"extracted_at,omitempty"`
		Confidence float64          `json:"confidence,omitempty"`
		Extractor  string           `json:"extractor,omitempty"`
	}
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, fmt.Errorf("unmarshal edge: %w", err)
	}
	e := &kgraph.Edge{
		ID:         flat.ID,
		Relation:   flat.Relation,
		Subject:    flat.Subject,
		Object:     flat.Object,
		Attributes: flat.Attributes,
		Provenance: kgraph.Provenance{
			Source:      flat.Source,
			ExtractedAt: flat.ExtractedAt,
			Confidence:  flat.Confidence,
			Extractor:   flat.Extractor,
		},
	}
	if e.Provenance.ExtractedAt.IsZero() {
		e.Provenance.ExtractedAt = time.Now()
	}
	return e, nil
}

func parseEvent(raw any) (*kgraph.Event, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	var flat struct {
		ID         string             `json:"id"`
		Type       string             `json:"type"`
		Trigger    string             `json:"trigger"`
		Time       time.Time          `json:"time,omitempty"`
		Arguments  []kgraph.EventArgument `json:"arguments,omitempty"`
		Attributes map[string]string  `json:"attributes,omitempty"`
		Source     string             `json:"source"`
		ExtractedAt time.Time        `json:"extracted_at,omitempty"`
		Confidence float64           `json:"confidence,omitempty"`
		Extractor  string            `json:"extractor,omitempty"`
	}
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	ev := &kgraph.Event{
		ID:         flat.ID,
		Type:       flat.Type,
		Trigger:    flat.Trigger,
		Time:       flat.Time,
		Arguments:  flat.Arguments,
		Attributes: flat.Attributes,
		Provenance: kgraph.Provenance{
			Source:      flat.Source,
			ExtractedAt: flat.ExtractedAt,
			Confidence:  flat.Confidence,
			Extractor:   flat.Extractor,
		},
	}
	if ev.Provenance.ExtractedAt.IsZero() {
		ev.Provenance.ExtractedAt = time.Now()
	}
	return ev, nil
}

// Compile-time interface check.
var _ tool.Tool = (*GraphTool)(nil)

func init() {
	tool.Register(NewGraphTool())
}
