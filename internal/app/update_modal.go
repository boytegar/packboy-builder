// Operation-mode cycling (Shift+Tab) and the question-modal protocol used
// by AskUserQuestion-style prompts surfaced from tools.
package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/app/input"
	"github.com/boytegar/packboy-builder/internal/log"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/tool"
)

func (m *model) cycleOperationMode() tea.Cmd {
	m.env.OperationMode = m.env.OperationMode.NextWithBypass(m.services.Setting.AllowBypass())
	m.applyOperationMode()
	m.persistOperationMode()
	// Landing on AutoPilot surfaces the opening proposal (Suggest) — but debounce
	// it: the cycle wraps through AutoPilot, so a gesture that only passes through
	// it must not fire a wasted LLM call. Confirm the user actually rested here
	// (handleAutopilotModeSettled). AutoPilot is no longer in the Shift+Tab cycle,
	// so this branch is only reached when /autopilot or /goal set the mode.
	if m.env.OperationMode == setting.ModeAutoPilot {
		return tea.Tick(autopilotSettleDelay, func(time.Time) tea.Msg { return autopilotModeSettledMsg{} })
	}
	return nil
}

// applyOperationMode re-applies the current mode's permission posture and hook
// mode — the shared tail of any mode change.
func (m *model) applyOperationMode() {
	m.env.ApplyModePermissions(m.env.CWD)
	m.services.Hook.SetPermissionMode(m.env.OperationModeName())
}

func (m *model) persistOperationMode() {
	if err := setting.UpdateLastOperationMode(m.env.OperationMode); err != nil {
		log.Logger().Warn("persist operation mode", zap.Error(err))
	}
}

// enterAutoPilotMode switches the operation posture to AutoPilot without cycling,
// so the /autopilot panel's Start button engages the copilot even from another
// mode (otherwise the kick's autopilotEngaged() gate would drop it).
func (m *model) enterAutoPilotMode() {
	m.env.OperationMode = setting.ModeAutoPilot
	m.applyOperationMode()
	m.persistOperationMode()
}

// applyYoloMode sets or toggles the orthogonal bypass-permissions flag from
// /yolo. Returns the notice shown to the user. Enable nil = toggle; non-nil =
// force on/off. Enabling is refused when allowBypass is false. Bypass is
// independent of OperationMode, so /yolo does not change the active mode
// (default/chat/agent) — it layers auto-accept on top of it.
func (m *model) applyYoloMode(enable *bool) string {
	currentlyOn := m.env.SessionPermissions.IsBypass()
	wantOn := enable != nil && *enable
	if enable == nil {
		wantOn = !currentlyOn
	}
	if wantOn {
		if !m.services.Setting.AllowBypass() {
			return "Bypass permissions is locked out (allowBypass: false in settings)."
		}
		if currentlyOn {
			return "Bypass permissions already on."
		}
		m.env.SessionPermissions.SetBypass(true)
		m.services.Hook.SetPermissionMode(m.env.OperationModeName())
		m.persistBypass(true)
		return "Bypass permissions on — tool calls auto-accepted. Active mode: " + m.env.OperationModeName() + " (shift+tab to cycle)."
	}
	if !currentlyOn {
		return "Bypass permissions already off."
	}
	m.env.SessionPermissions.SetBypass(false)
	m.services.Hook.SetPermissionMode(m.env.OperationModeName())
	m.persistBypass(false)
	return "Bypass permissions off. Active mode: " + m.env.OperationModeName() + "."
}

// persistBypass saves the bypass toggle to ~/.pcb/settings.json so it is
// restored on the next startup independent of the operation mode. Failures
// are non-fatal: the in-memory state already changed, only the persistence
// failed, so log and carry on.
func (m *model) persistBypass(on bool) {
	if err := setting.SaveBypassEnabled(on); err != nil {
		log.Logger().Warn("persist bypass toggle", zap.Bool("on", on), zap.Error(err))
	}
}

// autopilotSettleDelay is how long the mode must rest on AutoPilot before the
// kick/suggest fires — long enough to skip a pass-through cycle, short enough to
// feel immediate when the user deliberately stops here.
const autopilotSettleDelay = 250 * time.Millisecond

// autopilotModeSettledMsg fires autopilotSettleDelay after landing on AutoPilot.
type autopilotModeSettledMsg struct{}

// handleAutopilotModeSettled surfaces the Suggest steer's opening proposal once
// the mode has rested on AutoPilot. It no-ops if the user has since cycled away,
// so a pass-through cycle costs nothing. The mission kick-off is deliberately NOT
// here — that's the panel's explicit Start button, not a side effect of landing.
func (m *model) handleAutopilotModeSettled() tea.Cmd {
	if !m.autopilotEngaged() {
		return nil
	}
	return m.startPromptSuggestion()
}

func (m *model) updateMode(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case conv.QuestionRequestMsg:
		// Questions arrive via AgentToUI.Check(); re-arm the poll so the next
		// progress update or question keeps flowing while the modal is up.
		cmd := m.handleQuestionRequest(msg)
		if m.conv.AgentToUI != nil {
			cmd = tea.Batch(cmd, m.conv.AgentToUI.Check())
		}
		return cmd, true
	case conv.SecretPromptRequestMsg:
		cmd := m.handleSecretPromptRequest(msg)
		if m.conv.AgentToUI != nil {
			cmd = tea.Batch(cmd, m.conv.AgentToUI.Check())
		}
		return cmd, true
	}
	return nil, false
}

func (m *model) handleQuestionRequest(msg conv.QuestionRequestMsg) tea.Cmd {
	m.conv.Modal.PendingQuestion = msg.Request
	m.conv.Modal.PendingQuestionReply = msg.Reply
	m.conv.Modal.Question.Show(msg.Request, m.env.Width)
	cmds := m.CommitMessages()
	// Question steer (#4): show the modal, then let the copilot try to answer it
	// on the user's behalf (it defers back to the human when unsure).
	if cmd := m.autopilotAnswerQuestionCmd(msg.Request); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *model) handleQuestionResponse(msg conv.QuestionResponseMsg) tea.Cmd {
	reply := m.conv.Modal.PendingQuestionReply
	m.conv.Modal.PendingQuestionReply = nil
	defer func() { m.conv.Modal.PendingQuestion = nil }()

	if reply == nil {
		return nil
	}

	if msg.Cancelled {
		reply <- &tool.QuestionResponse{
			RequestID: msg.Request.ID,
			Cancelled: true,
		}
		return nil
	}
	reply <- msg.Response
	return nil
}

func (m *model) handleSecretPromptRequest(msg conv.SecretPromptRequestMsg) tea.Cmd {
	m.conv.Modal.PendingSecretReply = msg.Reply
	m.userInput.Secret.Show(msg.Prompt, m.env.Width)
	return nil
}

func (m *model) handleSecretPromptResponse(msg input.SecretPromptResponseMsg) tea.Cmd {
	reply := m.conv.Modal.PendingSecretReply
	m.conv.Modal.PendingSecretReply = nil
	if reply == nil {
		return nil
	}
	reply <- conv.SecretPromptResponse{
		Value:     msg.Value,
		Cancelled: msg.Cancelled,
	}
	return nil
}
