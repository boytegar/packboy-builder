package input

import (
	"fmt"
	"strings"
)

// RefineViewModel holds state for the /refine command view.
// It displays harness state summary, refinement history, and status.
type RefineViewModel struct {
	// Pending indicates a refinement is queued for the current turn.
	Pending bool
	// InFlight indicates a refinement is currently running.
	InFlight bool
	// Entries is a compact summary of harness entries by kind.
	Entries map[string]int
	// History shows recent refinement events.
	History []RefineHistoryItem
	// Mode: "status" (default), "history", "rollback"
	Mode string
}

// RefineHistoryItem is a single refinement event in the history list.
type RefineHistoryItem struct {
	ID      string
	Summary string
	Scope   string
	Edits   int
}

// NewRefineViewModel creates a default refine view model.
func NewRefineViewModel() *RefineViewModel {
	return &RefineViewModel{
		Entries: map[string]int{
			"prompt":   0,
			"memory":   0,
			"skill":    0,
			"subagent": 0,
		},
		Mode: "status",
	}
}

// Render produces the text output for the /refine command.
// This is displayed as a popup or inline message in the TUI.
func (m *RefineViewModel) Render() string {
	var b strings.Builder

	b.WriteString("✧ Continual Harness Refinement\n\n")

	switch m.Mode {
	case "status":
		b.WriteString(m.renderStatus())
	case "history":
		b.WriteString(m.renderHistory())
	case "rollback":
		b.WriteString("Rollback mode: use /refine rollback <id> to reverse a refinement.\n\n")
		b.WriteString(m.renderHistory())
	default:
		b.WriteString(m.renderStatus())
	}

	return b.String()
}

func (m *RefineViewModel) renderStatus() string {
	var b strings.Builder

	if m.InFlight {
		b.WriteString("Status: REFINING (a refinement is currently running)\n\n")
	} else if m.Pending {
		b.WriteString("Status: PENDING (refinement queued for end of turn)\n\n")
	} else {
		b.WriteString("Status: idle\n\n")
	}

	b.WriteString("Harness entries:\n")
	total := 0
	for _, kind := range []string{"prompt", "memory", "skill", "subagent"} {
		count := m.Entries[kind]
		total += count
		b.WriteString(fmt.Sprintf("  %s: %d\n", kind, count))
	}
	if total == 0 {
		b.WriteString("  (no entries yet — use /refine to create your first)\n")
	}

	b.WriteString("\nCommands:\n")
	b.WriteString("  /refine                    — trigger refinement\n")
	b.WriteString("  /refine <instructions>     — refine with focus instructions\n")
	b.WriteString("  /refine global             — target global (cross-session) store\n")
	b.WriteString("  /refine status             — show this status\n")
	b.WriteString("  /refine history            — show refinement history\n")
	b.WriteString("  /refine rollback <id>      — roll back a refinement\n")

	return b.String()
}

func (m *RefineViewModel) renderHistory() string {
	var b strings.Builder

	if len(m.History) == 0 {
		b.WriteString("No refinement history yet.\n")
		b.WriteString("Use /refine to trigger your first refinement.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Refinement history (%d events):\n\n", len(m.History)))
	for _, item := range m.History {
		scope := item.Scope
		if scope == "" {
			scope = "local"
		}
		b.WriteString(fmt.Sprintf("  [%s] %s (%s, %d edits)\n", item.ID, item.Summary, scope, item.Edits))
	}

	b.WriteString("\nUse /refine rollback <id> to reverse a refinement.\n")
	return b.String()
}

// ParseRefineArgs parses the arguments to /refine and returns the mode
// and any instructions or IDs.
func ParseRefineArgs(args string) (mode, instructions, rollbackID string, global bool) {
	args = strings.TrimSpace(args)
	if args == "" {
		return "trigger", "", "", false
	}

	parts := strings.SplitN(args, " ", 2)
	first := strings.ToLower(parts[0])

	switch first {
	case "status":
		return "status", "", "", false
	case "history":
		return "history", "", "", false
	case "rollback":
		if len(parts) > 1 {
			return "rollback", "", strings.TrimSpace(parts[1]), false
		}
		return "rollback", "", "", false
	case "global":
		rest := ""
		if len(parts) > 1 {
			rest = strings.TrimSpace(parts[1])
		}
		return "trigger", rest, "", true
	default:
		return "trigger", args, "", false
	}
}
