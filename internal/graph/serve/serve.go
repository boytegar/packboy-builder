// Package serve implements stage 9 of the knowledge-graph pipeline: serving
// the graph to LLMs via GraphRAG.
//
// The retrieval strategy per question type: entity lookup, k-hop traversal,
// subgraph extraction, or plain vector. Say which questions do not need the
// graph at all. If the graph doesn't win on multi-hop questions, it isn't
// earning its maintenance cost.
package serve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/boytegar/packboy-builder/internal/graph"
)

// ── Question types ──────────────────────────────────────────────

// QuestionType classifies a query for routing to the right retrieval method.
type QuestionType string

const (
	QTLookup      QuestionType = "lookup"       // single-hop: "what is X?"
	QTMultiHop   QuestionType = "multi_hop"    // "who worked with X on Y?"
	QTSubgraph   QuestionType = "subgraph"     // "everything related to X"
	QTVectorOnly QuestionType = "vector_only"  // does not need the graph
)

// ClassifyQuestion returns the question type for routing. This is the
// per-question-type strategy: say which questions do not need the graph.
func ClassifyQuestion(q string) QuestionType {
	q = strings.ToLower(q)
	// Multi-hop indicators: "who", "what ... with", "which ... for".
	if strings.Contains(q, "who ") || strings.Contains(q, "with ") || strings.Contains(q, "which ") || strings.Contains(q, " for ") {
		return QTMultiHop
	}
	// Lookup indicators: "what is", "define", "describe".
	if strings.Contains(q, "what is ") || strings.Contains(q, "define ") || strings.Contains(q, "describe ") {
		return QTLookup
	}
	// Subgraph indicators: "everything", "all", "related".
	if strings.Contains(q, "everything") || strings.Contains(q, "all ") || strings.Contains(q, "related") {
		return QTSubgraph
	}
	return QTVectorOnly
}

// ── Retrieval strategies ────────────────────────────────────────

// Retriever executes retrieval against the graph store.
type Retriever struct {
	Store *graph.Store
}

// NewRetriever creates a retriever bound to a graph store.
func NewRetriever(s *graph.Store) *Retriever {
	return &Retriever{Store: s}
}

// Retrieve performs the retrieval strategy for a question. Returns the
// serialized subgraph context and the strategy used.
func (r *Retriever) Retrieve(question string, k int) (string, QuestionType) {
	if r.Store == nil {
		return "", QTVectorOnly
	}
	qt := ClassifyQuestion(question)
	switch qt {
	case QTLookup:
		return r.entityLookup(question), qt
	case QTMultiHop:
		return r.kHopTraversal(question, k), qt
	case QTSubgraph:
		return r.subgraphExtract(question), qt
	default:
		return "", qt
	}
}

// entityLookup does a single-hop lookup by alias.
func (r *Retriever) entityLookup(question string) string {
	// Try to extract an entity name from the question (simplified).
	words := strings.Fields(question)
	for _, w := range words {
		w = strings.Trim(w, "?,.!\"'")
		if node, ok := r.Store.FindByAlias(w); ok {
			return serializeNode(node)
		}
	}
	return ""
}

// kHopTraversal does a k-hop traversal from the best-matching node.
func (r *Retriever) kHopTraversal(question string, k int) string {
	if k <= 0 {
		k = 2
	}
	// Find starting node.
	var start *graph.Node
	words := strings.Fields(question)
	for _, w := range words {
		w = strings.Trim(w, "?,.!\"'")
		if node, ok := r.Store.FindByAlias(w); ok {
			start = node
			break
		}
	}
	if start == nil {
		return ""
	}

	visited := make(map[string]struct{})
	var result []string
	queue := []string{start.ID}
	visited[start.ID] = struct{}{}
	depth := 0

	for len(queue) > 0 && depth < k {
		nextQueue := []string{}
		for _, nodeID := range queue {
			node, ok := r.Store.GetNode(nodeID)
			if !ok {
				continue
			}
			result = append(result, serializeNode(node))
			// Follow outgoing edges.
			for _, e := range r.Store.EdgesFrom(nodeID) {
				if _, seen := visited[e.Object]; !seen {
					visited[e.Object] = struct{}{}
					nextQueue = append(nextQueue, e.Object)
				}
			}
			// Follow incoming edges.
			for _, e := range r.Store.EdgesTo(nodeID) {
				if _, seen := visited[e.Subject]; !seen {
					visited[e.Subject] = struct{}{}
					nextQueue = append(nextQueue, e.Subject)
				}
			}
		}
		queue = nextQueue
		depth++
	}

	return strings.Join(result, "\n")
}

// subgraphExtract extracts the full neighborhood of the best-matching node.
func (r *Retriever) subgraphExtract(question string) string {
	return r.kHopTraversal(question, 3)
}

// ── Serialization ───────────────────────────────────────────────

// serializeNode converts a node to a compact context string. The serialization
// must not blow the window — keep it compact.
func serializeNode(n *graph.Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s (id=%s)", n.Type, n.Label, n.ID)
	if len(n.Attributes) > 0 {
		keys := make([]string, 0, len(n.Attributes))
		for k := range n.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(" {")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s=%s", k, n.Attributes[k])
		}
		b.WriteString("}")
	}
	if len(n.Aliases) > 0 {
		fmt.Fprintf(&b, " (aliases: %s)", strings.Join(n.Aliases, ", "))
	}
	return b.String()
}

// SerializeSubgraph serializes a subgraph for LLM context, respecting a token
// budget. Used to prevent blowing the window.
func SerializeSubgraph(nodes []*graph.Node, edges []*graph.Edge, tokenBudget int) string {
	var b strings.Builder
	b.WriteString("=== Subgraph ===\n")

	// Serialize nodes until budget is exhausted.
	nodeLines := make([]string, 0, len(nodes))
	for _, n := range nodes {
		nodeLines = append(nodeLines, serializeNode(n))
	}
	sort.Strings(nodeLines)

	estTokens := 0
	for _, line := range nodeLines {
		est := len(line) / 4 // rough token estimate
		if estTokens+est > tokenBudget {
			break
		}
		b.WriteString(line)
		b.WriteString("\n")
		estTokens += est
	}

	if len(edges) > 0 {
		b.WriteString("\n--- Relations ---\n")
		for _, e := range edges {
			line := fmt.Sprintf("%s --%s--> %s", e.Subject, e.Relation, e.Object)
			est := len(line) / 4
			if estTokens+est > tokenBudget {
				break
			}
			b.WriteString(line)
			b.WriteString("\n")
			estTokens += est
		}
	}

	return b.String()
}

// ── Vector baseline comparison ──────────────────────────────────

// VectorBaseline is a vector-only baseline over the same source text. Used for
// A/B comparison: if the graph doesn't win on multi-hop, it isn't earning its
// maintenance cost.
type VectorBaseline struct {
	SourceText string
	// SimulateResults holds the per-question results for the vector baseline.
	SimulateResults []BaselineResult
}

// BaselineResult holds one question's baseline performance.
type BaselineResult struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Correct   bool   `json:"correct"`
}

// ── Eval set ────────────────────────────────────────────────────

// EvalSet is the 30-question eval set written before either system runs.
type EvalSet struct {
	Items     []EvalItem `json:"items"`
	AnswerKey map[string]string `json:"answer_key"`
	Metric    string     `json:"metric"` // "exact_match", "f1", "hallucination_rate"
}

// EvalItem is one question in the eval set.
type EvalItem struct {
	ID        string `json:"id"`
	Question  string `json:"question"`
	Answer    string `json:"answer"` // gold answer, written before either system runs
	Type      QuestionType `json:"type"`
}

// NewEvalSet creates an eval set from items. The answer key is extracted from
// the items — it must be written before either system runs.
func NewEvalSet(items []EvalItem, metric string) *EvalSet {
	key := make(map[string]string, len(items))
	for _, item := range items {
		key[item.ID] = item.Answer
	}
	return &EvalSet{Items: items, AnswerKey: key, Metric: metric}
}

// Score scores a system's answers against the eval set.
func (es *EvalSet) Score(answers map[string]string) (correct, total int) {
	for id, gold := range es.AnswerKey {
		total++
		if ans, ok := answers[id]; ok && strings.EqualFold(strings.TrimSpace(ans), strings.TrimSpace(gold)) {
			correct++
		}
	}
	return correct, total
}

// Compare compares graph vs vector baseline. Returns true if graph wins.
func (es *EvalSet) Compare(graphAns, vectorAns map[string]string) bool {
	gCorrect, gTotal := es.Score(graphAns)
	vCorrect, vTotal := es.Score(vectorAns)
	if gTotal == 0 || vTotal == 0 {
		return false
	}
	gScore := float64(gCorrect) / float64(gTotal)
	vScore := float64(vCorrect) / float64(vTotal)
	return gScore > vScore
}
