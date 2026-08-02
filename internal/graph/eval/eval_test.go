package eval

import (
	"math/rand"
	"testing"
)

func TestComputeMetrics(t *testing.T) {
	samples := []Sample{
		{ID: "1", Predicted: "Person", Gold: "Person", IsCorrect: true},
		{ID: "2", Predicted: "Person", Gold: "Company", IsCorrect: false},
		{ID: "3", Predicted: "Company", Gold: "Company", IsCorrect: true},
		{ID: "4", Predicted: "Person", Gold: "Person", IsCorrect: true},
		{ID: "5", Predicted: "Company", Gold: "Person", IsCorrect: false},
	}
	m := ComputeMetrics(samples)
	if m.Total != 5 {
		t.Fatalf("expected total 5, got %d", m.Total)
	}
	if m.Correct != 3 {
		t.Fatalf("expected correct 3, got %d", m.Correct)
	}
	if m.Precision != 0.6 {
		t.Fatalf("expected precision 0.6, got %f", m.Precision)
	}
	if m.Confidence <= 0 {
		t.Error("confidence should be positive")
	}
}

func TestGatePass(t *testing.T) {
	gate := DefaultGate()

	// Passing metrics.
	pass := Metrics{Total: 50, Correct: 48, Precision: 0.96}
	if !gate.Pass(pass) {
		t.Error("gate should pass at 96% on 50 items")
	}

	// Failing on precision.
	fail := Metrics{Total: 50, Correct: 40, Precision: 0.80}
	if gate.Pass(fail) {
		t.Error("gate should fail at 80% precision")
	}

	// Failing on sample size.
	small := Metrics{Total: 10, Correct: 10, Precision: 1.0}
	if gate.Pass(small) {
		t.Error("gate should fail with sample size < 50")
	}
}

func TestGateVerdict(t *testing.T) {
	gate := DefaultGate()

	pass := Metrics{Total: 50, Correct: 48, Precision: 0.96, Confidence: 0.05}
	v := gate.Verdict(pass)
	if v == "" || v[:4] != "PASS" {
		t.Errorf("expected PASS verdict, got %s", v)
	}

	fail := Metrics{Total: 50, Correct: 40, Precision: 0.80}
	v = gate.Verdict(fail)
	if v == "" || v[:4] != "FAIL" {
		t.Errorf("expected FAIL verdict, got %s", v)
	}
}

func TestSampleN(t *testing.T) {
	pop := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	rng := rand.New(rand.NewSource(42))

	sample := SampleN(pop, 5, rng)
	if len(sample) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(sample))
	}

	// With n >= population, should return all.
	sample = SampleN(pop, 20, rng)
	if len(sample) != 10 {
		t.Fatalf("expected 10 (all), got %d", len(sample))
	}
}

func TestLeakageCheck(t *testing.T) {
	test := map[string]struct{}{
		"t1": {}, "t2": {}, "t3": {},
	}
	train := map[string]struct{}{
		"x1": {}, "t2": {}, // t2 leaks
	}
	lc := LeakageCheck{TestIDs: test, TrainIDs: train}
	if !lc.HasLeakage() {
		t.Error("should detect leakage")
	}
	overlap := lc.Overlap()
	if len(overlap) != 1 || overlap[0] != "t2" {
		t.Errorf("expected overlap [t2], got %v", overlap)
	}
}

func TestLinkPredictionEval(t *testing.T) {
	// Model beats trivial baseline.
	pass := LinkPredictionEval{Filtered: true, ModelHitsAtK: 0.85, TrivialBaselineHits: 0.20}
	if !pass.Passes() {
		t.Error("model should pass when it beats baseline")
	}

	// Model doesn't beat trivial baseline.
	fail := LinkPredictionEval{Filtered: true, ModelHitsAtK: 0.15, TrivialBaselineHits: 0.20}
	if fail.Passes() {
		t.Error("model should fail when below baseline")
	}

	summary := pass.Summary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}
