// Package task implements the task-graph half of graph engineering: how
// agents work, as opposed to what they remember.
//
// Nodes are jobs — each one something you would hand to a single assistant.
// Draw an arrow only when a job needs another job's result before it can start.
// The drawing is the plan; agents flow through it.
//
// Key patterns:
//   - Delete fake edges: an arrow is real only when work flows through it.
//   - The diamond: split → parallel workers → separate verifier contexts →
//     one owned merge.
//   - The stop rule: split only work that decomposes into independent pieces.
//     Sequential work stays with one agent.
//   - The human gate: put approval where a mistake is expensive to undo.
//
// Guardrails: max rounds per loop, one writer per file, routing in written
// steps, hard cap on agent spawn count.
package task

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ── Job (task-graph node) ───────────────────────────────────────

// Job is a node in the task graph — something you would hand to a single
// assistant (research one competitor, write one draft, check one claim).
type Job struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// AgentHint suggests which agent persona should handle this job.
	AgentHint string `json:"agent_hint,omitempty"`
	// Dependencies is the list of job IDs that must complete before this job.
	// An edge is real only when this job reads the previous job's result.
	Dependencies []string `json:"dependencies,omitempty"`
	// State travels with the work (what was found, what was decided, what remains).
	State map[string]string `json:"state,omitempty"`
	// Status tracks execution state.
	Status JobStatus `json:"status,omitempty"`
	// IsVerifier marks jobs that verify another job's output in a separate context.
	// The verification node is non-negotiable: a model grading its own work in
	// its own context misses most of its own mistakes.
	IsVerifier bool `json:"is_verifier,omitempty"`
	// VerifiesJob is the job ID this verifier checks (if IsVerifier).
	VerifiesJob string `json:"verifies_job,omitempty"`
	// HumanGate marks jobs that require explicit human approval before proceeding.
	// Put the gate where a mistake is expensive to undo, not on every step.
	HumanGate bool `json:"human_gate,omitempty"`
	// Irreversible marks jobs that perform hard-to-reverse actions (send,
	// publish, refund, delete, deploy). These must route through HumanGate.
	Irreversible bool `json:"irreversible,omitempty"`
	// Owner is the agent ID that owns the merge (for merge nodes). One owner
	// of the merge cuts error amplification from 17.2× to 4.4×.
	Owner string `json:"owner,omitempty"`
}

// JobStatus tracks the execution state of a job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobReady     JobStatus = "ready"     // all deps done, can start
	JobRunning   JobStatus = "running"
	JobVerifying JobStatus = "verifying"
	JobDone      JobStatus = "done"
	JobFailed    JobStatus = "failed"
	JobBlocked   JobStatus = "blocked" // blocked by human gate
)

// Validate checks that the job is well-formed. Irreversible jobs must have
// a human gate.
func (j *Job) Validate() error {
	if j.ID == "" {
		return errors.New("job: id is required")
	}
	if j.Name == "" {
		return fmt.Errorf("job %s: name is required", j.ID)
	}
	if j.Irreversible && !j.HumanGate {
		return fmt.Errorf("job %s: irreversible actions must route through a human gate", j.ID)
	}
	if j.IsVerifier && j.VerifiesJob == "" {
		return fmt.Errorf("job %s: verifier must specify which job it verifies", j.ID)
	}
	return nil
}

// ── TaskGraph (DAG) ─────────────────────────────────────────────

// TaskGraph is a DAG of jobs. Nodes are jobs, edges are execution dependencies.
// This is a DAG — the pattern that has run data infrastructure for decades
// (Airflow, Prefect, Temporal) now applied to agents (LangGraph, CrewAI, AutoGen).
type TaskGraph struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	// jobOrder preserves insertion order for stable iteration.
	jobOrder []string
}

// NewTaskGraph creates an empty task graph.
func NewTaskGraph() *TaskGraph {
	return &TaskGraph{
		jobs: make(map[string]*Job),
	}
}

// AddJob adds a job to the graph. The job is validated.
func (g *TaskGraph) AddJob(j *Job) error {
	if err := j.Validate(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.jobs[j.ID]; exists {
		return fmt.Errorf("job %s already exists", j.ID)
	}
	g.jobs[j.ID] = j
	g.jobOrder = append(g.jobOrder, j.ID)
	return nil
}

// GetJob retrieves a job by ID.
func (g *TaskGraph) GetJob(id string) (*Job, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	j, ok := g.jobs[id]
	return j, ok
}

// AllJobs returns all jobs in insertion order.
func (g *TaskGraph) AllJobs() []*Job {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Job, 0, len(g.jobOrder))
	for _, id := range g.jobOrder {
		result = append(result, g.jobs[id])
	}
	return result
}

// ── Fake edge detection ──────────────────────────────────────────

// FindFakeEdges returns dependency edges where the downstream job does not
// actually read the upstream job's result. "Summarize this file and then
// check my calendar" — the calendar step never uses the summary; the edge is
// fake. Delete fake edges and those jobs run in parallel.
//
// Detection: if a job's state does not reference any key from its dependency's
// state, the edge is likely fake.
func (g *TaskGraph) FindFakeEdges() []FakeEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var fakes []FakeEdge
	for _, job := range g.jobs {
		for _, depID := range job.Dependencies {
			dep, ok := g.jobs[depID]
			if !ok {
				continue
			}
			if !stateReferences(dep.State, job.State) {
				fakes = append(fakes, FakeEdge{
					From: depID,
					To:   job.ID,
					Reason: fmt.Sprintf("job %q does not reference any state from %q", job.ID, depID),
				})
			}
		}
	}
	return fakes
}

// FakeEdge represents a dependency edge that is likely fake.
type FakeEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// stateReferences returns true if any key in downstream appears in upstream's
// state values or keys.
func stateReferences(upstream, downstream map[string]string) bool {
	if len(upstream) == 0 || len(downstream) == 0 {
		// Can't determine — assume real to avoid false positives.
		return true
	}
	for k, v := range downstream {
		// Check if downstream key references upstream state.
		if _, ok := upstream[k]; ok {
			return true
		}
		// Check if downstream value references upstream state keys/values.
		for uk, uv := range upstream {
			if strings.Contains(v, uk) || strings.Contains(v, uv) {
				return true
			}
		}
	}
	return false
}

// ── Topological sort (execution order) ──────────────────────────

// TopoSort returns jobs in dependency order. Returns an error if the graph
// has a cycle (not a DAG).
func (g *TaskGraph) TopoSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Build in-degree map.
	inDegree := make(map[string]int, len(g.jobs))
	adj := make(map[string][]string, len(g.jobs))
	for _, id := range g.jobOrder {
		inDegree[id] = 0
	}
	for _, job := range g.jobs {
		for _, dep := range job.Dependencies {
			adj[dep] = append(adj[dep], job.ID)
			inDegree[job.ID]++
		}
	}

	// Kahn's algorithm.
	var queue []string
	for _, id := range g.jobOrder {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	var result []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		result = append(result, id)
		for _, next := range adj[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(result) != len(g.jobs) {
		return nil, errors.New("cycle detected — task graph is not a DAG")
	}
	return result, nil
}

// ── Parallel execution levels ───────────────────────────────────

// ParallelLevels returns jobs grouped by execution level. Jobs in the same
// level can run in parallel. This implements the diamond pattern: split into
// parallel workers, verify in separate contexts, merge survivors.
func (g *TaskGraph) ParallelLevels() ([][]string, error) {
	order, err := g.TopoSort()
	if err != nil {
		return nil, err
	}

	// Assign levels based on longest dependency chain.
	level := make(map[string]int, len(g.jobs))
	for _, id := range order {
		job := g.jobs[id]
		maxDepLevel := 0
		for _, dep := range job.Dependencies {
			if l, ok := level[dep]; ok && l >= maxDepLevel {
				maxDepLevel = l + 1
			}
		}
		level[id] = maxDepLevel
	}

	maxLevel := 0
	for _, l := range level {
		if l > maxLevel {
			maxLevel = l
		}
	}

	levels := make([][]string, maxLevel+1)
	for _, id := range order {
		levels[level[id]] = append(levels[level[id]], id)
	}

	// Sort within each level for stable output.
	for i := range levels {
		sort.Strings(levels[i])
	}
	return levels, nil
}

// ── Diamond pattern ─────────────────────────────────────────────

// Diamond represents the split → parallel workers → verify → merge pattern.
// The shape serious systems converge to.
type Diamond struct {
	Plan     string   `json:"plan"`     // the split point
	Workers  []string `json:"workers"`  // parallel worker job IDs
	Verifier string   `json:"verifier"`  // separate verifier context
	Merger   string   `json:"merger"`    // one owned merge
}

// Validate checks that the diamond is well-formed: has workers, a separate
// verifier, and one owned merge.
func (d Diamond) Validate() error {
	if d.Plan == "" {
		return errors.New("diamond: plan (split point) is required")
	}
	if len(d.Workers) == 0 {
		return errors.New("diamond: at least one worker is required")
	}
	if d.Verifier == "" {
		return errors.New("diamond: separate verifier context is non-negotiable")
	}
	if d.Merger == "" {
		return errors.New("diamond: one owned merge is required")
	}
	return nil
}

// FindDiamonds identifies diamond patterns in the task graph. A diamond is a
// plan job that fans out to ≥2 workers, and those workers all converge on a
// single merge job (the merge job depends on ≥2 of the workers).
// The verifier is detected if the merge job (or an intermediate job) is marked
// IsVerifier.
func (g *TaskGraph) FindDiamonds() []Diamond {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var diamonds []Diamond

	// Build reverse adjacency: for each job, which jobs depend on it?
	dependents := make(map[string][]string, len(g.jobs))
	for _, job := range g.jobs {
		for _, dep := range job.Dependencies {
			dependents[dep] = append(dependents[dep], job.ID)
		}
	}

	// For each job, check if it fans out (≥2 dependents) and those dependents
	// converge on a single job.
	for _, plan := range g.jobs {
		workers := dependents[plan.ID]
		if len(workers) < 2 {
			continue
		}

		// Find the convergence point: a job whose dependencies include ≥2 of
		// the workers (excluding plan).
		workerSet := make(map[string]struct{}, len(workers))
		for _, w := range workers {
			workerSet[w] = struct{}{}
		}

		var merger, verifier string
		for _, job := range g.jobs {
			if job.ID == plan.ID {
				continue
			}
			overlapCount := 0
			for _, dep := range job.Dependencies {
				if _, ok := workerSet[dep]; ok {
					overlapCount++
				}
			}
			if overlapCount >= 2 {
				merger = job.ID
				if job.IsVerifier {
					verifier = job.ID
				}
				break
			}
		}

		if merger != "" {
			diamonds = append(diamonds, Diamond{
				Plan:     plan.ID,
				Workers:  workers,
				Verifier: verifier,
				Merger:   merger,
			})
		}
	}
	return diamonds
}

// ── Stop rule analysis ──────────────────────────────────────────

// StopRuleAnalysis applies the stop rule: split only work that decomposes into
// independent pieces. Sequential work stays with one agent.
//
// From Google DeepMind × MIT (180 configurations): coordinated teams beat a
// single agent by ~80% on work that splits; every multi-agent configuration
// lost on sequential work (degrading 39-70%). Uncoordinated agents amplified
// errors 17.2×; a single coordinator owning the merge cut it to 4.4×.
type StopRuleAnalysis struct {
	SplittableJobs []string `json:"splittable_jobs"`
	SequentialJobs []string `json:"sequential_jobs"`
	Recommendation string   `json:"recommendation"`
}

// AnalyzeStopRule splits jobs into splittable (independent) and sequential
// (needs full picture). More agents is not a strategy; the shape of the work
// decides.
func (g *TaskGraph) AnalyzeStopRule() StopRuleAnalysis {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var splittable, sequential []string
	for _, job := range g.jobs {
		if len(job.Dependencies) == 0 {
			splittable = append(splittable, job.ID)
		} else {
			sequential = append(sequential, job.ID)
		}
	}

	rec := "parallel"
	if len(sequential) > len(splittable) {
		rec = "single_agent"
	}

	return StopRuleAnalysis{
		SplittableJobs: splittable,
		SequentialJobs: sequential,
		Recommendation: rec,
	}
}

// ── Guardrails ──────────────────────────────────────────────────

// Guardrails are the four caps that keep a graph from becoming an expensive
// accident.
type Guardrails struct {
	MaxRoundsPerLoop  int `json:"max_rounds_per_loop"`  // 1. Every loop gets a maximum
	MaxConcurrentAgents int `json:"max_concurrent_agents"` // 4. A hard cap on how many agents can spawn
}

// DefaultGuardrails returns sensible defaults for the four caps.
func DefaultGuardrails() Guardrails {
	return Guardrails{
		MaxRoundsPerLoop:    10,
		MaxConcurrentAgents: 8,
	}
}

// ── Human gate routing ──────────────────────────────────────────

// HumanGates returns all jobs that require human approval. These are the
// irreversible edges — send, publish, refund, delete, deploy — that must
// route through explicit approval.
func (g *TaskGraph) HumanGates() []*Job {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var gates []*Job
	for _, id := range g.jobOrder {
		if g.jobs[id].HumanGate {
			gates = append(gates, g.jobs[id])
		}
	}
	return gates
}

// PendingGates returns human-gate jobs that are currently blocked, awaiting
// approval.
func (g *TaskGraph) PendingGates() []*Job {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var pending []*Job
	for _, id := range g.jobOrder {
		j := g.jobs[id]
		if j.HumanGate && j.Status == JobBlocked {
			pending = append(pending, j)
		}
	}
	return pending
}

// ── Summary ──────────────────────────────────────────────────────

// Summary returns a human-readable summary of the task graph.
func (g *TaskGraph) Summary() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "TaskGraph: %d jobs\n", len(g.jobs))

	fakes := g.FindFakeEdges()
	if len(fakes) > 0 {
		fmt.Fprintf(&b, "\n⚠️  %d fake edge(s) detected — delete to parallelize:\n", len(fakes))
		for _, fe := range fakes {
			fmt.Fprintf(&b, "  %s → %s: %s\n", fe.From, fe.To, fe.Reason)
		}
	}

	diamonds := g.FindDiamonds()
	if len(diamonds) > 0 {
		fmt.Fprintf(&b, "\n💎 %d diamond pattern(s):\n", len(diamonds))
		for _, d := range diamonds {
			fmt.Fprintf(&b, "  plan=%s → workers=%v → verifier=%s → merge=%s\n",
				d.Plan, d.Workers, d.Verifier, d.Merger)
		}
	}

	gates := g.HumanGates()
	if len(gates) > 0 {
		fmt.Fprintf(&b, "\n🔒 %d human gate(s):\n", len(gates))
		for _, gate := range gates {
			fmt.Fprintf(&b, "  %s: %s\n", gate.ID, gate.Name)
		}
	}

	analysis := g.AnalyzeStopRule()
	fmt.Fprintf(&b, "\nStop rule: %d splittable, %d sequential → %s\n",
		len(analysis.SplittableJobs), len(analysis.SequentialJobs), analysis.Recommendation)

	return b.String()
}
