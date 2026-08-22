package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/agent"
	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/hook"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/todo"
)

// modelForCtrlC wires only what the Ctrl+C paths touch: the agent session that
// QuitWithCancel/ResetAgentSession stop, the hook engine + settings that
// FireSessionEnd needs for its budget, and the task tracker /clear resets.
func modelForCtrlC(t *testing.T) *model {
	t.Helper()
	data := setting.NewData()
	return &model{
		conv: conv.NewModel(80),
		services: services{
			Agent:   &agent.Session{},
			Hook:    hook.NewEngine(data, "test-session", t.TempDir(), ""),
			Setting: setting.New(data),
			Tracker: todo.NewStore(),
		},
	}
}

func isQuit(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestCtrlCQuitsWhenConversationIsEmpty(t *testing.T) {
	m := modelForCtrlC(t)

	cmd, handled := m.handleTextareaShortcut(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !handled {
		t.Fatal("Ctrl+C was not handled")
	}
	if !isQuit(t, cmd) {
		t.Fatal("Ctrl+C on an empty conversation should quit, not clear")
	}
}

func TestCtrlCClearsBeforeQuittingWhenConversationHasMessages(t *testing.T) {
	m := modelForCtrlC(t)
	m.conv.Append(core.ChatMessage{Role: core.RoleUser, Content: "hello"})

	cmd, handled := m.handleTextareaShortcut(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !handled {
		t.Fatal("Ctrl+C was not handled")
	}
	if isQuit(t, cmd) {
		t.Fatal("the first Ctrl+C on a non-empty conversation should clear, not quit")
	}
	if len(m.conv.Messages) != 0 {
		t.Fatalf("conv messages = %d, want 0 after clear", len(m.conv.Messages))
	}

	// The conversation is empty now, so the follow-up tap quits without needing
	// to land inside the double-tap window.
	cmd, handled = m.handleTextareaShortcut(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !handled {
		t.Fatal("second Ctrl+C was not handled")
	}
	if !isQuit(t, cmd) {
		t.Fatal("Ctrl+C after the conversation was cleared should quit")
	}
}
