// /tokenlimit selector: pick which agent context to budget (main agent,
// sub-agent, or write sub-agent), then enter that context's input and output
// token limits. The values persist to the global settings.json as
// mainTokenLimit / agentTokenLimit and drive the runtime resolver
// (llm.Store.EffectiveInputLimitFor + the output-cap override).
//
// The old behaviour — auto-fetching the current model's limits and setting a
// per-model override in providers.json — is gone. This overlay replaces it.
package input

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/setting"
)

// TokenLimitSavedMsg is emitted when the /tokenlimit overlay persists a
// role-scoped budget (empty input+output means it was cleared). The app shows
// it as a status-line notice.
type TokenLimitSavedMsg struct {
	Target string
	Input  int
	Output int
}

// tokenLimitTarget identifies which context the user is budgeting.
type tokenLimitTarget int

const (
	targetMain tokenLimitTarget = iota
	targetAgent
	targetAgentWrite
)

func (t tokenLimitTarget) label() string {
	switch t {
	case targetAgent:
		return "Sub-agent"
	case targetAgentWrite:
		return "Sub-agent (write)"
	default:
		return "Main agent"
	}
}

// roleKey returns the settings-role key used by setting.UpdateTokenLimitFor.
func (t tokenLimitTarget) roleKey() string {
	switch t {
	case targetAgent, targetAgentWrite:
		return "agent"
	default:
		return "main"
	}
}

// describe returns the user-facing line for the input context.
func (t tokenLimitTarget) describe() string {
	switch t {
	case targetAgent:
		return "read-only sub-agents"
	case targetAgentWrite:
		return "write-enabled sub-agents"
	default:
		return "the main conversation agent"
	}
}

// currentOverride reads the currently persisted /tokenlimit override for the
// target from the live settings snapshot, so the editor is prefilled.
func (t tokenLimitTarget) currentOverride() setting.TokenLimitOverride {
	d := setting.Default()
	if t.roleKey() == "agent" {
		return d.AgentTokenLimit()
	}
	return d.MainTokenLimit()
}

// tokenLimitField identifies the numeric field being edited.
type tokenLimitField int

const (
	fieldInput tokenLimitField = iota
	fieldOutput
)

// TokenLimitSelector is the /tokenlimit overlay: a compact 3-row target picker
// flowing into a two-field numeric editor. The target list and the numeric
// editor are two stages of one overlay; Esc retreats the editor to the list.
type TokenLimitSelector struct {
	// keep the deps needed to persist. userLevel is always true (global
	// ~/.pcb/settings.json) — the user budgets the whole tool, not one project.
	userLevel bool

	active    bool
	target    tokenLimitTarget
	nav       kit.ListNav
	stage     int // 0 = pick target, 1 = edit numbers
	field     tokenLimitField
	value     [2]string // input, output
	errorText string
	width     int
	height    int
}

func NewTokenLimitSelector() TokenLimitSelector {
	return TokenLimitSelector{userLevel: true}
}

func (s *TokenLimitSelector) IsActive() bool { return s.active }

func (s *TokenLimitSelector) panel() kit.Panel { return kit.Panel{Width: s.width, Height: s.height} }

// EnterSelect opens the overlay at the target picker, prefilled from the
// current settings so the user can review before editing.
func (s *TokenLimitSelector) EnterSelect(width, height int) *TokenLimitSelector {
	s.width = width
	s.height = height
	s.active = true
	s.stage = 0
	s.field = fieldInput
	s.errorText = ""
	s.nav.Total = 3
	s.nav.MaxVisible = 3
	s.nav.ResetCursor()
	return s
}

// Cancel closes the overlay entirely.
func (s *TokenLimitSelector) Cancel() {
	s.active = false
	s.stage = 0
}

// selectTarget advances to the numeric editor for the highlighted target.
func (s *TokenLimitSelector) selectTarget() {
	t := tokenLimitTarget(s.nav.Selected)
	s.target = t
	s.stage = 1
	s.field = fieldInput
	s.errorText = ""
	cur := t.currentOverride()
	if cur.InputTokenLimit > 0 {
		s.value[0] = strconv.Itoa(cur.InputTokenLimit)
	} else {
		s.value[0] = ""
	}
	if cur.OutputTokenLimit > 0 {
		s.value[1] = strconv.Itoa(cur.OutputTokenLimit)
	} else {
		s.value[1] = ""
	}
}

// parseValues validates the two numeric fields.
func (s *TokenLimitSelector) parseValues() (int, int, error) {
	in, err1 := strconv.Atoi(strings.TrimSpace(s.value[0]))
	out, err2 := strconv.Atoi(strings.TrimSpace(s.value[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("both limits must be whole numbers")
	}
	if in < 0 || out < 0 {
		return 0, 0, fmt.Errorf("limits cannot be negative")
	}
	return in, out, nil
}

// commit persists the edited limits for the selected target and returns a
// message the app turns into a notice. Zero clears the override for the role.
func (s *TokenLimitSelector) commit() tea.Cmd {
	in, out, err := s.parseValues()
	if err != nil {
		s.errorText = err.Error()
		return nil
	}
	tl := setting.TokenLimitOverride{InputTokenLimit: in, OutputTokenLimit: out}
	if err := setting.UpdateTokenLimitFor(s.target.roleKey(), tl, s.userLevel); err != nil {
		s.errorText = "failed to save: " + err.Error()
		return nil
	}
	s.active = false
	s.stage = 0
	return func() tea.Msg {
		return TokenLimitSavedMsg{Target: s.target.label(), Input: in, Output: out}
	}
}

// HandleKeypress routes keys for the active overlay.
func (s *TokenLimitSelector) HandleKeypress(key tea.KeyMsg) tea.Cmd {
	if !s.active {
		return nil
	}
	if s.stage == 1 {
		return s.handleKeyEdit(key)
	}
	return s.handleKeyTarget(key)
}

func (s *TokenLimitSelector) handleKeyTarget(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "up", "k", "ctrl+p":
		s.nav.MoveUp()
	case "down", "j", "ctrl+n":
		s.nav.MoveDown()
	case "enter":
		s.selectTarget()
	case "esc":
		s.Cancel()
		return func() tea.Msg { return kit.DismissedMsg{} }
	}
	return nil
}

func (s *TokenLimitSelector) handleKeyEdit(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		// Retreat to the target list, keeping the current selection.
		s.stage = 0
		s.errorText = ""
	case "tab", "down":
		if s.field == fieldInput {
			s.field = fieldOutput
		} else {
			s.field = fieldInput
		}
	case "shift+tab", "up":
		if s.field == fieldOutput {
			s.field = fieldInput
		} else {
			s.field = fieldOutput
		}
	case "enter":
		return s.commit()
	case "backspace":
		v := s.value[s.field]
		if len(v) > 0 {
			s.value[s.field] = v[:len(v)-1]
		}
		s.errorText = ""
	default:
		if text := key.Key().Text; text != "" {
			for _, r := range text {
				if r >= '0' && r <= '9' {
					s.value[s.field] += string(r)
				}
			}
			s.errorText = ""
		}
	}
	return nil
}

// Render draws the active overlay: the target list or the numeric editor.
func (s *TokenLimitSelector) Render() string {
	if !s.active {
		return ""
	}
	panel := s.panel()
	var sb strings.Builder
	sb.WriteString(panel.SeparatorLine())
	sb.WriteString("\n")
	sb.WriteString(kit.SelectorTitleStyle().Render("Token Limits"))
	sb.WriteString("\n\n")

	if s.stage == 1 {
		sb.WriteString(s.renderEditor(panel))
	} else {
		sb.WriteString(s.renderTargets(panel))
	}

	sb.WriteString("\n")
	sb.WriteString(panel.SeparatorLine())
	sb.WriteString("\n")
	if s.stage == 1 {
		if s.errorText != "" {
			errStyle := lipgloss.NewStyle().Foreground(kit.AdaptiveColor{Dark: "#F87171", Light: "#DC2626"})
			sb.WriteString(errStyle.Render(s.errorText))
		} else {
			sb.WriteString(kit.DimStyle().Render("Tab/↑↓ switch field · Enter save · Esc back"))
		}
	} else {
		sb.WriteString(kit.DimStyle().Render("↑/↓ choose context · Enter edit · Esc close"))
	}
	return panel.Wrap(sb.String())
}

func (s *TokenLimitSelector) renderTargets(panel kit.Panel) string {
	var sb strings.Builder
	targets := []tokenLimitTarget{targetMain, targetAgent, targetAgentWrite}
	for i, t := range targets {
		cur := t.currentOverride()
		meta := "unset"
		if cur.InputTokenLimit > 0 || cur.OutputTokenLimit > 0 {
			meta = fmt.Sprintf("in %s · out %s",
				kit.FormatTokenCount(cur.InputTokenLimit), kit.FormatTokenCount(cur.OutputTokenLimit))
		}
		line := kit.FormatAlignedRow("", t.label(), 22, kit.DimStyle().Render(meta))
		sb.WriteString(kit.RenderSelectableRow(line, i == s.nav.Selected))
		sb.WriteString("\n")
	}
	return panel.PadViewport(sb.String())
}

func (s *TokenLimitSelector) renderEditor(panel kit.Panel) string {
	var sb strings.Builder
	accent := lipgloss.NewStyle().Foreground(kit.CurrentTheme.Accent)
	sb.WriteString(accent.Render(s.target.label()))
	sb.WriteString(kit.DimStyle().Render(" — limits for " + s.target.describe() + ".\n\n"))
	for _, f := range []tokenLimitField{fieldInput, fieldOutput} {
		label := "Input tokens"
		if f == fieldOutput {
			label = "Output tokens"
		}
		val := s.value[f]
		if val == "" {
			val = "0"
		}
		marker := "  "
		style := lipgloss.NewStyle()
		if s.field == f {
			marker = kit.FocusBarStyle().Render(kit.FocusBar) + " "
			style = style.Bold(true)
		}
		sb.WriteString(marker + style.Render(fmt.Sprintf("%-15s %s", label, val)) + "\n")
	}
	return panel.PadViewport(sb.String())
}
