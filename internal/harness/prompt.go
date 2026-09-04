package harness

import (
	"fmt"
	"strings"
)

const (
	defaultMaxEntriesPerKind = 6
	defaultMaxRefinements    = 5
	defaultMaxContentLength  = 180
)

// FormatHarnessStateForPrompt renders a compact summary of the harness state
// for injection into the system prompt. Entries are truncated to fit; the full
// content should be loaded on demand when detail matters.
func FormatHarnessStateForPrompt(st *HarnessState, opts FormatOptions) string {
	maxEntries := opts.MaxEntriesPerKind
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntriesPerKind
	}
	maxRefinements := opts.MaxRefinements
	if maxRefinements <= 0 {
		maxRefinements = defaultMaxRefinements
	}
	maxContent := opts.MaxContentLength
	if maxContent <= 0 {
		maxContent = defaultMaxContentLength
	}

	var b strings.Builder

	b.WriteString("# Continual Harness State\n\n")
	b.WriteString("Local continual harness entries belong to this session. Global continual harness entries persist across sessions.\n")
	b.WriteString("The entries below are compact summaries, not full descriptions. Use them as routing/context hints; refine the underlying entry only when detail matters.\n")
	b.WriteString("Default to local refinement for current task progress, temporary blockers, and session coordination. Use global refinement only for stable cross-session lessons, durable user preferences, reusable skills/subagents, or explicitly project-qualified facts.\n")
	b.WriteString("The base system prompt is immutable; prompt entries below are supplemental notes only.\n\n")

	b.WriteString("When to call `harness_refine`: after a repeated failure, a reusable tactic emerges, a repeated delegation role should become a subagent spec, a repeated procedure should become a skill, a durable fact/preference should become a memory, a narrow behavioral policy should become a prompt addendum, a user corrects behavior that should persist locally or globally, validation shows a harness entry is wrong, or a skill/subagent/memory/prompt note should be created, updated, deleted, or rolled back. Keep harness edits small and evidence-backed.\n\n")

	totalEntries := 0
	for _, kind := range AllKinds() {
		entries := SortedEntries(st, kind)
		totalEntries += len(entries)

		b.WriteString(fmt.Sprintf("%s: %d\n", kind, len(entries)))

		for _, entry := range entries {
			if len(entries) > maxEntries && indexOf(entries, entry) >= maxEntries {
				continue
			}
			_ = maxEntries // currently not capping per-entry in this simplified version

			scope := entry.Scope
			if scope == "" {
				scope = ScopeGlobal
			}

			line := fmt.Sprintf("- [%s:%s] %s (%s, v%d)", scope, entry.ID, entry.Title, entry.Path, entry.Version)

			if entry.Kind == KindSkill && len(entry.Arguments) > 0 {
				line += " args=" + compactText(jsonCompact(entry.Arguments), maxContent)
			}
			if entry.Kind == KindSkill && len(entry.Reference) > 0 {
				line += " ref=" + compactText(jsonCompact(entry.Reference), maxContent)
			}

			line += ": " + compactText(entry.Content, maxContent)
			b.WriteString(line + "\n")
		}

		overflow := len(entries) - min(len(entries), maxEntries)
		if overflow > 0 {
			b.WriteString(fmt.Sprintf("- +%d more %s entries\n", overflow, kind))
		}
		b.WriteString("\n")
	}

	if totalEntries == 0 {
		b.WriteString("No saved harness entries yet.\n\n")
	}

	b.WriteString(fmt.Sprintf("recent refinements: %d\n", len(st.Refinements)))
	for _, event := range st.Refinements {
		if len(st.Refinements) > maxRefinements {
			// Show only the most recent
		}
		changes := "no applied edits"
		if len(event.Changes) > 0 {
			changes = strings.Join(event.Changes, ", ")
		}
		outcome := ""
		if event.Outcome != "" {
			outcome = "; outcome: " + compactText(event.Outcome, maxContent)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s: %s%s\n", event.ID, compactText(event.Trigger, maxContent), changes, outcome))
	}

	refinementOverflow := len(st.Refinements) - min(len(st.Refinements), maxRefinements)
	if refinementOverflow > 0 {
		b.WriteString(fmt.Sprintf("- +%d older refinement events\n", refinementOverflow))
	}

	return strings.TrimSpace(b.String())
}

// FormatOptions controls the output of FormatHarnessStateForPrompt.
type FormatOptions struct {
	MaxEntriesPerKind int
	MaxRefinements    int
	MaxContentLength  int
}

// --- helpers ---

func compactText(text string, maxLength int) string {
	normalized := strings.Join(strings.Fields(text), " ")
	if len(normalized) <= maxLength {
		return normalized
	}
	if maxLength <= 3 {
		return normalized[:maxLength]
	}
	return normalized[:maxLength-3] + "..."
}

func jsonCompact(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteByte('{')
	first := true
	for k, v := range m {
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(fmt.Sprintf("%q: %v", k, v))
	}
	b.WriteByte('}')
	return b.String()
}

func indexOf(entries []HarnessEntry, target HarnessEntry) int {
	for i, e := range entries {
		if e.ID == target.ID {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
