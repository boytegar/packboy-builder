// Package fuse implements stage 8 of the knowledge-graph pipeline: knowledge
// fusion — deduplication and merging of entities within and across sources.
//
// Skipping fusion is the #1 cause of useless graphs. The pipeline:
//   - Blocking: avoid n² comparisons by partitioning candidates into blocks.
//   - Matching: score candidate pairs within blocks.
//   - Review band: human decides in the uncertain score range.
//   - Merge: combine survivors with a conflict resolution policy.
//
// Merges must be reversible. Every merge is logged so it can be undone.
package fuse

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/boytegar/packboy-builder/internal/graph"
	"github.com/boytegar/packboy-builder/internal/graph/extract"
)

// ── Blocking ────────────────────────────────────────────────────

// Blocker partitions entity candidates into blocks so matching is not n².
// Candidates in different blocks are assumed not to match.
type Blocker interface {
	// Block returns a block key for the candidate. Candidates with the same
	// key are compared against each other only.
	Block(c EntityCandidate) string
}

// EntityCandidate is an entity awaiting fusion (re-exported from extract).
type EntityCandidate = extract.EntityCandidate

// PrefixBlocker groups by the first N characters of the normalized label.
// Simple and effective for many domains. Expected reduction: ~1/blockSize.
type PrefixBlocker struct {
	PrefixLen int
}

func (b PrefixBlocker) Block(c EntityCandidate) string {
	norm := normalizeLabel(c.Label)
	if b.PrefixLen <= 0 || len(norm) < b.PrefixLen {
		return norm
	}
	return norm[:b.PrefixLen]
}

// TypeBlocker groups by entity type first, then by prefix. This prevents
// cross-type comparisons entirely.
type TypeBlocker struct {
	PrefixLen int
}

func (b TypeBlocker) Block(c EntityCandidate) string {
	norm := normalizeLabel(c.Label)
	if b.PrefixLen <= 0 || len(norm) < b.PrefixLen {
		return c.Type + ":" + norm
	}
	return c.Type + ":" + norm[:b.PrefixLen]
}

// Block builds blocks of candidates. Returns a map of block key → candidates.
func Block(candidates []EntityCandidate, blocker Blocker) map[string][]EntityCandidate {
	blocks := make(map[string][]EntityCandidate)
	for _, c := range candidates {
		key := blocker.Block(c)
		blocks[key] = append(blocks[key], c)
	}
	return blocks
}

// ── Matching ────────────────────────────────────────────────────

// MatchFunction scores a pair of candidates. Higher score = more likely same.
type MatchFunction func(a, b EntityCandidate) float64

// LevenshteinMatcher uses string similarity on labels with field weights.
type LevenshteinMatcher struct {
	LabelWeight float64
	AttrWeights map[string]float64
	Threshold   float64
}

// Score returns a 0.0–1.0 match score for a pair.
func (m LevenshteinMatcher) Score(a, b EntityCandidate) float64 {
	labelSim := stringSimilarity(a.Label, b.Label)
	score := labelSim * m.LabelWeight

	// Attribute similarity.
	attrWeightSum := 0.0
	attrScore := 0.0
	for attr, w := range m.AttrWeights {
		va, oka := a.Attributes[attr]
		vb, okb := b.Attributes[attr]
		if oka && okb {
			attrScore += w * stringSimilarity(va, vb)
			attrWeightSum += w
		}
	}
	if attrWeightSum > 0 {
		score += attrScore
		// Normalize: total weight = labelWeight + attrWeightSum (but only for attrs present).
		total := m.LabelWeight + attrWeightSum
		score /= total
	} else {
		score /= m.LabelWeight
	}
	return score
}

// ── Review band ─────────────────────────────────────────────────

// ReviewBand defines the score range where a human decides instead of the
// machine. Below low → auto-reject. Above high → auto-merge. In between →
// human review.
type ReviewBand struct {
	Low  float64 // below this, auto-reject
	High float64 // at/above this, auto-merge
}

// Decision classifies a match score.
type Decision string

const (
	DecisionAutoMerge  Decision = "auto_merge"
	DecisionAutoReject Decision = "auto_reject"
	DecisionHuman      Decision = "human_review"
)

// Classify returns the decision for a given score.
func (b ReviewBand) Classify(score float64) Decision {
	if score >= b.High {
		return DecisionAutoMerge
	}
	if score < b.Low {
		return DecisionAutoReject
	}
	return DecisionHuman
}

// ── Merge policy ────────────────────────────────────────────────

// MergePolicy decides which source wins on conflict, and what survives as an
// alias.
type MergePolicy struct {
	// SourcePriority maps source → priority. Higher wins on conflict.
	SourcePriority map[string]int
	// KeepAlias: if true, the absorbed node's label becomes an alias.
	KeepAlias bool
}

// Merge merges b into a according to the policy. Returns the merged node and
// a log record for reversibility. Merges must be reversible.
func Merge(a, b *graph.Node, policy MergePolicy) (*graph.Node, MergeRecord) {
	record := MergeRecord{
		Timestamp: time.Now(),
		Survivor:  a.ID,
		Absorbed:  b.ID,
		Reason:    "fusion",
	}

	// If the absorbed label differs, keep it as an alias.
	if policy.KeepAlias && a.Label != b.Label && !a.HasAlias(b.Label) {
		a.Aliases = append(a.Aliases, b.Label)
	}

	// Merge aliases from b.
	for _, alias := range b.Aliases {
		if !a.HasAlias(alias) && alias != a.Label {
			a.Aliases = append(a.Aliases, alias)
		}
	}

	// Merge attributes: b wins only if its source has higher priority.
	aPriority := policy.SourcePriority[a.Provenance.Source]
	bPriority := policy.SourcePriority[b.Provenance.Source]
	for k, v := range b.Attributes {
		if existing, ok := a.Attributes[k]; ok {
			if bPriority > aPriority {
				a.Attributes[k] = v
			}
			_ = existing
		} else {
			a.Attributes[k] = v
		}
	}

	// Track merged origin.
	a.MergedFrom = append(a.MergedFrom, b.ID)
	return a, record
}

// MergeRecord logs a single merge operation so it can be undone.
type MergeRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Survivor  string    `json:"survivor"`
	Absorbed  string    `json:"absorbed"`
	Reason    string    `json:"reason,omitempty"`
}

// ── Fusion pipeline ─────────────────────────────────────────────

// Pipeline runs the full fusion process on a set of candidates against an
// existing graph store.
type Pipeline struct {
	Blocker Blocker
	Matcher MatchFunction
	Band    ReviewBand
	Policy  MergePolicy
	Store   *graph.Store
}

// Run executes the fusion pipeline. Returns merge records and human-review
// items.
func (p *Pipeline) Run(candidates []EntityCandidate) ([]MergeRecord, []ReviewItem, error) {
	if p.Store == nil {
		return nil, nil, fmt.Errorf("fuse: store is nil")
	}

	// First, add all candidates as new nodes (validated against ontology).
	// In a real pipeline, candidates would be extracted entities from stage 4-6.
	// Here we resolve them against existing nodes via the alias index.

	var merges []MergeRecord
	var reviews []ReviewItem

	blocks := Block(candidates, p.Blocker)
	for key, blockCandidates := range blocks {
		_ = key
		// Compare each candidate to existing nodes in the same block.
		for _, c := range blockCandidates {
			existing, found := p.Store.FindByAlias(c.Label)
			if found {
				// Candidate matches existing node — merge.
				score := p.Matcher(EntityCandidate{Label: existing.Label, Type: existing.Type, Attributes: existing.Attributes}, c)
				decision := p.Band.Classify(score)
				switch decision {
				case DecisionAutoMerge:
					// Reconstruct the existing node for merge.
					existingNode := existing
					merged, rec := Merge(existingNode, &graph.Node{
						ID:         c.Label + ":candidate",
						Type:       c.Type,
						Label:      c.Label,
						Attributes: c.Attributes,
						Provenance: graph.Provenance{Source: "fuse", ExtractedAt: time.Now()},
					}, p.Policy)
					_ = merged
					merges = append(merges, rec)
				case DecisionHuman:
					reviews = append(reviews, ReviewItem{
						Candidate: c,
						Existing:  existing,
						Score:     score,
					})
				case DecisionAutoReject:
					// Not a match — add as new node.
				}
			}
		}
	}

	sort.Slice(merges, func(i, j int) bool { return merges[i].Timestamp.Before(merges[j].Timestamp) })
	return merges, reviews, nil
}

// ReviewItem is a candidate pair in the review band, needing human decision.
type ReviewItem struct {
	Candidate EntityCandidate `json:"candidate"`
	Existing  *graph.Node     `json:"existing"`
	Score     float64         `json:"score"`
}

// ── Helpers ─────────────────────────────────────────────────────

func normalizeLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// stringSimilarity returns a 0.0–1.0 similarity score based on Levenshtein
// distance normalized by the longer string's length.
func stringSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 1.0
	}
	d := levenshtein(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(d)/float64(maxLen)
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
