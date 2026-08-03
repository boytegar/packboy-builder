// Package extract implements stages 4-6 of the knowledge-graph pipeline:
// entity extraction (NER), relation extraction, and event extraction.
//
// The method ladder: exact rules/dictionaries for closed vocabularies → LLM
// extraction with the ontology in the prompt for open text. Always extract
// with span + source pointer for provenance.
//
// Extraction is stage machinery, not the pipeline. The surrounding schema,
// validation, and fusion are what make the output a knowledge graph.
package extract

import (
	"fmt"
	"strings"
	"time"

	"github.com/boytegar/packboy-builder/internal/graph"
)

// ── Source router (stage 4 entry) ───────────────────────────────

// SourceKind classifies a data source for extraction routing. The first two
// kinds (structured, semi-structured) should not need a model.
type SourceKind string

const (
	SourceStructured     SourceKind = "structured"      // tables, JSON, CSV
	SourceSemiStructured SourceKind = "semi-structured" // HTML, XML
	SourceUnstructured   SourceKind = "unstructured"    // PDF, plain text
)

// Source describes a single input to the extraction pipeline.
type Source struct {
	Kind   SourceKind `json:"kind"`
	Path   string     `json:"path"`             // file path, URL, or table identifier
	Format string     `json:"format,omitempty"` // "csv", "json", "html", "pdf", "txt"
}

// ExtractionResult holds the output of one extraction pass on a source.
type ExtractionResult struct {
	Source      Source              `json:"source"`
	Entities    []EntityCandidate   `json:"entities"`
	Relations   []RelationCandidate `json:"relations,omitempty"`
	Events      []EventCandidate    `json:"events,omitempty"`
	Errors      []string            `json:"errors,omitempty"`
	ExtractedAt time.Time           `json:"extracted_at"`
}

// EntityCandidate is an extracted entity awaiting validation + fusion.
type EntityCandidate struct {
	Type       string            `json:"type"`
	Label      string            `json:"label"`
	Span       string            `json:"span"` // verbatim evidence
	Attributes map[string]string `json:"attributes,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
}

// RelationCandidate is an extracted typed triple awaiting validation.
type RelationCandidate struct {
	Relation     string  `json:"relation"`
	SubjectLabel string  `json:"subject_label"`
	ObjectLabel  string  `json:"object_label"`
	Span         string  `json:"span"`
	Confidence   float64 `json:"confidence,omitempty"`
}

// EventCandidate is an extracted event awaiting validation.
type EventCandidate struct {
	Type      string              `json:"type"`
	Trigger   string              `json:"trigger"`
	Span      string              `json:"span"`
	Time      time.Time           `json:"time,omitempty"`
	Arguments []EventArgCandidate `json:"arguments,omitempty"`
}

// EventArgCandidate is a role-filled participant in an extracted event.
type EventArgCandidate struct {
	Role  string `json:"role"`
	Label string `json:"label"` // entity label, resolved later
}

// Router splits sources by kind and routes to the appropriate method.
// Structured and semi-structured sources do not need a model.
type Router struct {
	ontology *graph.Ontology
}

// NewRouter creates an extraction router bound to the given ontology.
func NewRouter(o *graph.Ontology) *Router {
	return &Router{ontology: o}
}

// Classify determines the SourceKind for a given path/format.
func (r *Router) Classify(source Source) SourceKind {
	if source.Kind != "" {
		return source.Kind
	}
	switch strings.ToLower(source.Format) {
	case "csv", "json", "sql", "parquet":
		return SourceStructured
	case "html", "xml", "yaml":
		return SourceSemiStructured
	default:
		return SourceUnstructured
	}
}

// ── Validation against ontology ─────────────────────────────────

// ValidateEntities filters entity candidates against the ontology, returning
// only those whose type is declared. Entities with unknown types are
// collected as errors.
func (r *Router) ValidateEntities(candidates []EntityCandidate) (valid []EntityCandidate, errs []string) {
	for _, c := range candidates {
		if r.ontology.EntityType(c.Type) == nil {
			errs = append(errs, fmt.Sprintf("entity %q: type %q not in ontology", c.Label, c.Type))
			continue
		}
		valid = append(valid, c)
	}
	return valid, errs
}

// ValidateRelations filters relation candidates against the ontology's
// domain/range constraints. This one validation step removes most hallucinated
// structure. Relations whose endpoints have incompatible types are rejected.
func (r *Router) ValidateRelations(candidates []RelationCandidate, resolvedTypes map[string]string) (valid []RelationCandidate, errs []string) {
	for _, c := range candidates {
		rel := r.ontology.Relation(c.Relation)
		if rel == nil {
			errs = append(errs, fmt.Sprintf("relation %q: not in ontology", c.Relation))
			continue
		}
		subjectType, ok := resolvedTypes[c.SubjectLabel]
		if !ok {
			errs = append(errs, fmt.Sprintf("relation %q: subject %q not resolved", c.Relation, c.SubjectLabel))
			continue
		}
		objectType, ok := resolvedTypes[c.ObjectLabel]
		if !ok {
			errs = append(errs, fmt.Sprintf("relation %q: object %q not resolved", c.Relation, c.ObjectLabel))
			continue
		}
		if !r.ontology.RelationValidFor(c.Relation, subjectType, objectType) {
			// Allow subclass substitution.
			if !(r.ontology.IsA(subjectType, rel.Domain) && r.ontology.IsA(objectType, rel.Range)) {
				errs = append(errs, fmt.Sprintf("relation %q: requires %s→%s, got %s→%s",
					c.Relation, rel.Domain, rel.Range, subjectType, objectType))
				continue
			}
		}
		valid = append(valid, c)
	}
	return valid, errs
}

// ── Rejection rules (pre-fusion filtering) ──────────────────────

// RejectionRule is a predicate that decides whether a candidate should be
// dropped before it reaches the graph.
type RejectionRule func(interface{}) bool

// RejectLowConfidence returns a rule that drops candidates below a threshold.
func RejectLowConfidence(threshold float64) RejectionRule {
	return func(c interface{}) bool {
		switch v := c.(type) {
		case EntityCandidate:
			return v.Confidence < threshold
		case RelationCandidate:
			return v.Confidence < threshold
		}
		return false
	}
}

// RejectEmptySpan returns a rule that drops candidates with no evidence span.
// A triple with no evidence span is a hallucination with extra steps.
func RejectEmptySpan() RejectionRule {
	return func(c interface{}) bool {
		switch v := c.(type) {
		case EntityCandidate:
			return strings.TrimSpace(v.Span) == ""
		case RelationCandidate:
			return strings.TrimSpace(v.Span) == ""
		}
		return false
	}
}

// ApplyRejectionRules filters candidates through a list of rejection rules.
func ApplyRejectionRules(candidates []RelationCandidate, rules []RejectionRule) (kept []RelationCandidate, rejected int) {
	for _, c := range candidates {
		drop := false
		for _, rule := range rules {
			if rule(c) {
				drop = true
				rejected++
				break
			}
		}
		if !drop {
			kept = append(kept, c)
		}
	}
	return kept, rejected
}

// ── Failure modes ───────────────────────────────────────────────

// FailureMode describes a likely extraction failure and how to detect it.
type FailureMode struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	DetectionCheck string `json:"detection_check"`
}

// CommonFailureModes returns the 5 failure modes most likely for the given
// source kind. These are detection prompts, not fixes.
func CommonFailureModes(kind SourceKind) []FailureMode {
	switch kind {
	case SourceStructured:
		return []FailureMode{
			{Name: "type mismatch", Description: "entity type in row does not match ontology", DetectionCheck: "compare row type column to ontology.EntityType"},
			{Name: "missing identifier", Description: "row lacks the field that uniquely identifies an instance", DetectionCheck: "check EntityType.Identifies field presence"},
			{Name: "duplicate rows", Description: "same entity appears multiple times", DetectionCheck: "count distinct IDs vs row count"},
			{Name: "null attributes", Description: "required attributes are empty", DetectionCheck: "check EntityType.Attributes[*].Required"},
			{Name: "encoding errors", Description: "non-UTF8 bytes corrupt labels", DetectionCheck: "validate UTF-8 on read"},
		}
	case SourceSemiStructured:
		return []FailureMode{
			{Name: "selector drift", Description: "HTML structure changed, selectors miss data", DetectionCheck: "compare extracted count to previous run"},
			{Name: "inline scripts", Description: "JS-generated content not in static HTML", DetectionCheck: "check if selector matches > 0 but text empty"},
			{Name: "encoding", Description: "charset declaration wrong", DetectionCheck: "compare declared charset to detected"},
			{Name: "merged cells", Description: "table layout splits attributes across rows", DetectionCheck: "check rowspan/colspan"},
			{Name: "boilerplate", Description: "nav/footer text extracted as entities", DetectionCheck: "filter against XPath allowlist"},
		}
	default: // unstructured
		return []FailureMode{
			{Name: "hallucinated entities", Description: "model emits entities not in source text", DetectionCheck: "check if span verbatim appears in source"},
			{Name: "type confusion", Description: "entity assigned wrong type", DetectionCheck: "compare to ontology type definitions"},
			{Name: "span drift", Description: "evidence span doesn't match the entity", DetectionCheck: "check if span contains label"},
			{Name: "boundary errors", Description: "entity label cut off or merged", DetectionCheck: "check span starts/ends at word boundaries"},
			{Name: "low confidence", Description: "model unsure, emits garbage", DetectionCheck: "filter confidence < 0.7"},
		}
	}
}

// ── Hand-check protocol (50-document sample) ─────────────────────

// HandCheckProtocol defines the 50-document sampling protocol.
type HandCheckProtocol struct {
	SampleSize    int      `json:"sample_size"`
	WhatToRecord  string   `json:"what_to_record"`
	StopThreshold string   `json:"stop_threshold"`
	Fields        []string `json:"fields"`
}

// DefaultHandCheckProtocol returns the standard 50-document protocol.
func DefaultHandCheckProtocol() HandCheckProtocol {
	return HandCheckProtocol{
		SampleSize:    50,
		WhatToRecord:  "for each sampled document: entity precision (real + correctly typed), relation precision (source asserts edge), span accuracy",
		StopThreshold: "entity precision ≥90% AND relation precision ≥90% — stop tuning, proceed to fusion",
		Fields:        []string{"doc_id", "entity_precision", "relation_precision", "span_accuracy", "errors"},
	}
}
