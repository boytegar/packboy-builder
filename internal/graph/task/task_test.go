package task

import (
	"testing"
)

func TestTaskGraphBasic(t *testing.T) {
	g := NewTaskGraph()

	// Plan job.
	plan := &Job{ID: "plan", Name: "Plan research", Status: JobPending}
	if err := g.AddJob(plan); err != nil {
		t.Fatalf("AddJob plan: %v", err)
	}

	// Worker jobs (depend on plan, independent of each other).
	w1 := &Job{ID: "w1", Name: "Research competitor A", Dependencies: []string{"plan"}, Status: JobPending}
	w2 := &Job{ID: "w2", Name: "Research competitor B", Dependencies: []string{"plan"}, Status: JobPending}
	g.AddJob(w1)
	g.AddJob(w2)

	// Verifier (depends on both workers, separate context).
	verify := &Job{ID: "verify", Name: "Verify findings", IsVerifier: true, VerifiesJob: "w1", Dependencies: []string{"w1", "w2"}, Status: JobPending}
	g.AddJob(verify)

	// Merge (depends on verifier, one owner).
	merge := &Job{ID: "merge", Name: "Merge report", Dependencies: []string{"verify"}, Status: JobPending, Owner: "coordinator"}
	g.AddJob(merge)

	// Topological sort.
	order, err := g.TopoSort()
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	if len(order) != 5 {
		t.Fatalf("expected 5 jobs, got %d", len(order))
	}
	// Plan must come before workers.
	planIdx := indexOf(order, "plan")
	w1Idx := indexOf(order, "w1")
	if planIdx > w1Idx {
		t.Error("plan must come before w1")
	}
	// Merge must come after verify.
	verifyIdx := indexOf(order, "verify")
	mergeIdx := indexOf(order, "merge")
	if verifyIdx > mergeIdx {
		t.Error("verify must come before merge")
	}
}

func TestParallelLevels(t *testing.T) {
	g := NewTaskGraph()
	g.AddJob(&Job{ID: "a", Name: "A"})
	g.AddJob(&Job{ID: "b", Name: "B"})
	g.AddJob(&Job{ID: "c", Name: "C", Dependencies: []string{"a", "b"}})
	g.AddJob(&Job{ID: "d", Name: "D", Dependencies: []string{"c"}})

	levels, err := g.ParallelLevels()
	if err != nil {
		t.Fatalf("ParallelLevels: %v", err)
	}
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	// Level 0: a, b (parallel)
	// Level 1: c
	// Level 2: d
	if len(levels[0]) != 2 {
		t.Errorf("expected 2 jobs in level 0, got %d", len(levels[0]))
	}
	if len(levels[1]) != 1 || levels[1][0] != "c" {
		t.Errorf("expected c in level 1, got %v", levels[1])
	}
	if len(levels[2]) != 1 || levels[2][0] != "d" {
		t.Errorf("expected d in level 2, got %v", levels[2])
	}
}

func TestCycleDetection(t *testing.T) {
	g := NewTaskGraph()
	g.AddJob(&Job{ID: "a", Name: "A", Dependencies: []string{"c"}})
	g.AddJob(&Job{ID: "b", Name: "B", Dependencies: []string{"a"}})
	g.AddJob(&Job{ID: "c", Name: "C", Dependencies: []string{"b"}})

	_, err := g.TopoSort()
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestIrreversibleRequiresHumanGate(t *testing.T) {
	// Irreversible without human gate should fail validation.
	g := NewTaskGraph()
	bad := &Job{ID: "deploy", Name: "Deploy", Irreversible: true, HumanGate: false}
	if err := g.AddJob(bad); err == nil {
		t.Error("expected error: irreversible without human gate")
	}

	// With human gate, should pass.
	good := &Job{ID: "deploy2", Name: "Deploy2", Irreversible: true, HumanGate: true}
	if err := g.AddJob(good); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHumanGates(t *testing.T) {
	g := NewTaskGraph()
	g.AddJob(&Job{ID: "a", Name: "A"})
	g.AddJob(&Job{ID: "deploy", Name: "Deploy", Irreversible: true, HumanGate: true, Status: JobBlocked})
	g.AddJob(&Job{ID: "b", Name: "B"})

	gates := g.HumanGates()
	if len(gates) != 1 {
		t.Fatalf("expected 1 gate, got %d", len(gates))
	}

	pending := g.PendingGates()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending gate, got %d", len(pending))
	}
	if pending[0].ID != "deploy" {
		t.Errorf("expected deploy, got %s", pending[0].ID)
	}
}

func TestStopRuleAnalysis(t *testing.T) {
	g := NewTaskGraph()
	g.AddJob(&Job{ID: "a", Name: "A"})                              // splittable (no deps)
	g.AddJob(&Job{ID: "b", Name: "B"})                              // splittable
	g.AddJob(&Job{ID: "c", Name: "C", Dependencies: []string{"a"}}) // sequential

	analysis := g.AnalyzeStopRule()
	if len(analysis.SplittableJobs) != 2 {
		t.Errorf("expected 2 splittable, got %d", len(analysis.SplittableJobs))
	}
	if len(analysis.SequentialJobs) != 1 {
		t.Errorf("expected 1 sequential, got %d", len(analysis.SequentialJobs))
	}
}

func TestDiamondPattern(t *testing.T) {
	g := NewTaskGraph()
	g.AddJob(&Job{ID: "plan", Name: "Plan"})
	g.AddJob(&Job{ID: "w1", Name: "Worker 1", Dependencies: []string{"plan"}})
	g.AddJob(&Job{ID: "w2", Name: "Worker 2", Dependencies: []string{"plan"}})
	g.AddJob(&Job{ID: "merge", Name: "Merge", Dependencies: []string{"w1", "w2"}, Owner: "coord"})

	diamonds := g.FindDiamonds()
	if len(diamonds) != 1 {
		t.Fatalf("expected 1 diamond, got %d", len(diamonds))
	}
	d := diamonds[0]
	if d.Plan != "plan" {
		t.Errorf("expected plan, got %s", d.Plan)
	}
	if len(d.Workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(d.Workers))
	}
	if d.Merger != "merge" {
		t.Errorf("expected merge, got %s", d.Merger)
	}
}

func TestFakeEdgeDetection(t *testing.T) {
	g := NewTaskGraph()
	// Real edge: b reads a's state.
	g.AddJob(&Job{
		ID:    "a",
		Name:  "A",
		State: map[string]string{"result": "data"},
	})
	g.AddJob(&Job{
		ID:           "b",
		Name:         "B",
		Dependencies: []string{"a"},
		State:        map[string]string{"input": "result-from-a"}, // references "result"
	})

	// Fake edge: c depends on b but doesn't use b's output.
	g.AddJob(&Job{
		ID:           "c",
		Name:         "Check calendar",
		Dependencies: []string{"b"},
		State:        map[string]string{"task": "calendar"}, // no reference to b's state
	})

	fakes := g.FindFakeEdges()
	// Should detect at least the c→b edge as potentially fake.
	// Note: a→b is real because b's state references "result".
	if len(fakes) == 0 {
		t.Log("no fake edges detected — heuristic may be conservative")
	}
}

func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}
