package conv

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/core"
)

type postToolRuntime struct {
	Runtime
	drainCalls int
}

func (r *postToolRuntime) OnToolResult(tr core.ToolResult) *core.ToolResult { return &tr }
func (r *postToolRuntime) TakeDecision(string) *core.ReviewDecision         { return nil }
func (r *postToolRuntime) DrainQueuedAtStep() tea.Cmd {
	r.drainCalls++
	return nil
}

func TestPostToolDrainsQueuedInputAfterEntireToolBatch(t *testing.T) {
	m := NewModel(80)
	m.Tool.Track([]core.ToolCall{{ID: "tc-1", Name: "Read"}, {ID: "tc-2", Name: "Bash"}})
	rt := &postToolRuntime{}

	applyPostTool(rt, &m, core.PostToolEvent(core.ToolResult{ToolCallID: "tc-1", ToolName: "Read"}))
	if rt.drainCalls != 0 {
		t.Fatalf("drained pending input after first tool result; calls = %d", rt.drainCalls)
	}
	applyPostTool(rt, &m, core.PostToolEvent(core.ToolResult{ToolCallID: "tc-2", ToolName: "Bash"}))
	if rt.drainCalls != 1 {
		t.Fatalf("drain calls after complete tool batch = %d, want 1", rt.drainCalls)
	}
}

func TestHandleActivityWithoutAgentToUIDoesNotPanic(t *testing.T) {
	m := OutputModel{Spinner: newFrameClock(), MDRenderer: NewMDRenderer(80)}

	cmd := m.HandleActivity(AgentActivityMsg{
		Index:   1,
		Message: "step",
	})
	if cmd == nil {
		t.Fatal("expected spinner cmd even without an agent-to-UI channel")
	}
	if len(m.TaskActivity[1]) != 1 || m.TaskActivity[1][0] != "step" {
		t.Fatalf("unexpected activity state: %#v", m.TaskActivity)
	}
}

func Test_drainActivityWithoutHubIsNoop(t *testing.T) {
	m := OutputModel{Spinner: newFrameClock(), MDRenderer: NewMDRenderer(80)}
	m.TaskActivity = map[int][]string{2: {"existing"}}

	m.drainActivity()

	if len(m.TaskActivity[2]) != 1 || m.TaskActivity[2][0] != "existing" {
		t.Fatalf("unexpected activity state after drain: %#v", m.TaskActivity)
	}
}

func TestMarkToolCallCompleteAdvancesAndClearsPendingState(t *testing.T) {
	state := ToolExecState{}
	state.Track([]core.ToolCall{
		{ID: "tc-1", Name: "WebFetch"},
		{ID: "tc-2", Name: "Grep"},
	})

	state.MarkCurrent("tc-1")
	if state.CurrentIdx != 0 {
		t.Fatalf("CurrentIdx = %d, want 0", state.CurrentIdx)
	}

	if complete := state.MarkComplete("tc-1"); complete {
		t.Fatal("first tool must not complete the batch")
	}
	if state.CurrentIdx != 1 {
		t.Fatalf("CurrentIdx = %d, want 1", state.CurrentIdx)
	}
	if len(state.PendingCalls) != 2 {
		t.Fatalf("PendingCalls length = %d, want 2", len(state.PendingCalls))
	}

	if complete := state.MarkComplete("tc-2"); !complete {
		t.Fatal("last tool must complete the batch")
	}
	if state.PendingCalls != nil {
		t.Fatalf("PendingCalls = %#v, want nil", state.PendingCalls)
	}
	if state.CurrentIdx != 0 {
		t.Fatalf("CurrentIdx = %d, want 0", state.CurrentIdx)
	}
}

// Out-of-order completion (tc-2 before tc-1) must not clear the batch early —
// that was the parallel-mode bug where finishing the last-indexed call wiped
// pending state while earlier calls were still running.
func TestMarkToolCallCompleteOutOfOrder(t *testing.T) {
	state := ToolExecState{}
	state.Track([]core.ToolCall{
		{ID: "tc-1", Name: "Agent"},
		{ID: "tc-2", Name: "Read"},
	})

	if complete := state.MarkComplete("tc-2"); complete {
		t.Fatal("completing last-indexed call first must not finish the batch")
	}
	if len(state.PendingCalls) != 2 {
		t.Fatalf("PendingCalls length = %d, want 2", len(state.PendingCalls))
	}
	// Cursor stays on tc-1 until it completes.
	if state.CurrentIdx != 0 {
		t.Fatalf("CurrentIdx = %d, want 0 (tc-1 still running)", state.CurrentIdx)
	}

	if complete := state.MarkComplete("tc-1"); !complete {
		t.Fatal("batch must complete once every call is done")
	}
	if state.PendingCalls != nil {
		t.Fatalf("PendingCalls = %#v, want nil", state.PendingCalls)
	}
}

func TestDrainPendingCallsSkipsAlreadyCompleted(t *testing.T) {
	state := ToolExecState{}
	state.Track([]core.ToolCall{
		{ID: "tc-1", Name: "Agent"},
		{ID: "tc-2", Name: "Read"},
		{ID: "tc-3", Name: "Bash"},
	})
	state.MarkComplete("tc-2")

	remaining := state.DrainPendingCalls()
	if len(remaining) != 2 {
		t.Fatalf("remaining = %d, want 2 (tc-1, tc-3)", len(remaining))
	}
	if remaining[0].ID != "tc-1" || remaining[1].ID != "tc-3" {
		t.Fatalf("remaining IDs = %s,%s want tc-1,tc-3", remaining[0].ID, remaining[1].ID)
	}
	if state.PendingCalls != nil {
		t.Fatalf("state not reset after drain: %#v", state.PendingCalls)
	}
}

func TestPostToolOutOfOrderDoesNotDrainEarly(t *testing.T) {
	m := NewModel(80)
	m.Tool.Track([]core.ToolCall{{ID: "tc-1", Name: "Agent"}, {ID: "tc-2", Name: "Read"}})
	rt := &postToolRuntime{}

	applyPostTool(rt, &m, core.PostToolEvent(core.ToolResult{ToolCallID: "tc-2", ToolName: "Read"}))
	if rt.drainCalls != 0 {
		t.Fatalf("drained after out-of-order first completion; calls = %d", rt.drainCalls)
	}
	if len(m.Tool.PendingCalls) != 2 {
		t.Fatalf("PendingCalls cleared early: len=%d", len(m.Tool.PendingCalls))
	}
	applyPostTool(rt, &m, core.PostToolEvent(core.ToolResult{ToolCallID: "tc-1", ToolName: "Agent"}))
	if rt.drainCalls != 1 {
		t.Fatalf("drain calls after full batch = %d, want 1", rt.drainCalls)
	}
}
