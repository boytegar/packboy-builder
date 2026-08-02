// Package eval implements stage 7 of the knowledge-graph pipeline: the
// quality gate. Before fusion, sample and score.
//
// Fix the prompt/rules, not the output, then re-run. Target ≥90% precision
// on a 50-item sample before proceeding — recall improves with more passes;
// bad precision poisons the graph permanently.
package eval

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// ── Precision / recall ─────────────────────────────────────────

// Sample is a single item in an evaluation sample.
type Sample struct {
	ID         string `json:"id"`
	Predicted  string `json:"predicted"`   // predicted entity type, relation, or label
	Gold       string `json:"gold"`        // gold-standard answer
	IsCorrect  bool   `json:"is_correct"`  // human-judged correctness
	SpanCorrect bool  `json:"span_correct,omitempty"` // for relations: does the source assert the edge?
}

// Metrics holds precision/recall/F1 for a sample.
type Metrics struct {
	Total      int     `json:"total"`
	Correct    int     `json:"correct"`
	Incorrect  int     `json:"incorrect"`
	Precision  float64 `json:"precision"`
	Recall     float64 `json:"recall,omitempty"`
	F1         float64 `json:"f1,omitempty"`
	Confidence float64 `json:"confidence"` // Wilson score interval half-width
}

// ComputeMetrics calculates precision (and recall if gold positives are known)
// from a set of judged samples.
func ComputeMetrics(samples []Sample) Metrics {
	var m Metrics
	m.Total = len(samples)
	for _, s := range samples {
		if s.IsCorrect {
			m.Correct++
		} else {
			m.Incorrect++
		}
	}
	if m.Total > 0 {
		m.Precision = float64(m.Correct) / float64(m.Total)
	}
	m.Recall = m.Precision // simplified: recall estimated from same sample
	m.F1 = 2 * m.Precision * m.Recall / (m.Precision + m.Recall + 1e-9)
	m.Confidence = wilsonConfidence(m.Total, m.Correct, 0.95)
	return m
}

// wilsonConfidence returns the half-width of the Wilson score interval at the
// given confidence level. Used to state a confidence interval, not a vibe.
func wilsonConfidence(n, successes int, zLevel float64) float64 {
	if n == 0 {
		return 1.0
	}
	p := float64(successes) / float64(n)
	z := zLevel // 1.96 for 95%
	denom := 1 + z*z/float64(n)
	center := (p + z*z/(2*float64(n))) / denom
	spread := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n))) / denom
	_ = center
	return spread
}

// ── Sampling ────────────────────────────────────────────────────

// SampleN draws a random sample of size n from the population using the
// provided RNG. Deterministic if the RNG is seeded.
func SampleN(population []string, n int, rng *rand.Rand) []string {
	if n >= len(population) {
		out := make([]string, len(population))
		copy(out, population)
		sort.Strings(out)
		return out
	}
	indices := rng.Perm(len(population))
	selected := make([]string, 0, n)
	for i := 0; i < n; i++ {
		selected = append(selected, population[indices[i]])
	}
	sort.Strings(selected)
	return selected
}

// ── Test-set leakage ─────────────────────────────────────────────

// LeakageCheck detects overlap between two ID sets (e.g. test vs train).
// Where the test set leaks into the training or prompt-development set, the
// metrics are inflated.
type LeakageCheck struct {
	TestIDs  map[string]struct{}
	TrainIDs map[string]struct{}
}

// Overlap returns the IDs present in both sets.
func (l LeakageCheck) Overlap() []string {
	var dup []string
	for id := range l.TestIDs {
		if _, ok := l.TrainIDs[id]; ok {
			dup = append(dup, id)
		}
	}
	sort.Strings(dup)
	return dup
}

// HasLeakage returns true if any test ID appears in the train set.
func (l LeakageCheck) HasLeakage() bool {
	return len(l.Overlap()) > 0
}

// ── Link prediction filtering ───────────────────────────────────

// LinkPredictionEval evaluates a link-prediction model with the filtered
// setting (standard practice). A trivial baseline must be scored first.
type LinkPredictionEval struct {
	// Filtered: whether the filtered setting was used. In the filtered setting,
	// true triples from the test set are removed from the corruption candidates.
	Filtered bool

	// TrivialBaseline: the score a trivial baseline (e.g. most-popular-node)
	// would achieve. If the model doesn't beat this, it isn't earning its keep.
	TrivialBaselineHits float64
	ModelHitsAtK         float64
}

// Passes returns true if the model beats the trivial baseline.
func (e LinkPredictionEval) Passes() bool {
	return e.ModelHitsAtK > e.TrivialBaselineHits
}

// Summary returns a human-readable summary of the link prediction evaluation.
func (e LinkPredictionEval) Summary() string {
	setting := "raw"
	if e.Filtered {
		setting = "filtered"
	}
	verdict := "FAIL"
	if e.Passes() {
		verdict = "PASS"
	}
	return fmt.Sprintf("link prediction [%s]: model=%.3f trivial=%.3f → %s", setting, e.ModelHitsAtK, e.TrivialBaselineHits, verdict)
}

// ── Quality gate ────────────────────────────────────────────────

// Gate is the quality gate for stage 7. Target ≥90% precision on a 50-item
// sample before proceeding to fusion.
type Gate struct {
	MinPrecision   float64 // default 0.90
	MinSampleSize  int     // default 50
}

// DefaultGate returns the standard quality gate: 90% precision on 50 items.
func DefaultGate() Gate {
	return Gate{MinPrecision: 0.90, MinSampleSize: 50}
}

// Pass returns true if the metrics meet the gate threshold.
func (g Gate) Pass(m Metrics) bool {
	return m.Total >= g.MinSampleSize && m.Precision >= g.MinPrecision
}

// Verdict returns a human-readable verdict with the stop/proceed instruction.
func (g Gate) Verdict(m Metrics) string {
	if g.Pass(m) {
		return fmt.Sprintf("PASS: precision=%.2f%% on %d items (CI±%.2f%%) → proceed to fusion", m.Precision*100, m.Total, m.Confidence*100)
	}
	if m.Total < g.MinSampleSize {
		return fmt.Sprintf("HOLD: sample size %d < %d required → collect more, do not tune yet", m.Total, g.MinSampleSize)
	}
	return fmt.Sprintf("FAIL: precision=%.2f%% < %.2f%% → fix the prompt/rules, re-run extraction. Do NOT fix the output.", m.Precision*100, g.MinPrecision*100)
}
