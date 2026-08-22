package input

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/setting"
)

func keyPress(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		// A printable rune: match on the first char.
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
}

// The /tokenlimit overlay opens at the target list and renders all three
// targets; navigating down and pressing enter moves to the numeric editor.
func TestTokenLimitSelectorNavigatesToEditor(t *testing.T) {
	s := NewTokenLimitSelector()
	s.EnterSelect(80, 24)

	if !s.IsActive() {
		t.Fatal("selector should be active after EnterSelect")
	}
	view := s.Render()
	for _, want := range []string{"Main agent", "Sub-agent", "Sub-agent (write)", "Token Limits"} {
		if !strings.Contains(view, want) {
			t.Errorf("target view missing %q", want)
		}
	}

	// Down twice → Sub-agent (write) selected, enter → editor stage.
	s.HandleKeypress(keyPress("down"))
	s.HandleKeypress(keyPress("down"))
	if cmd := s.HandleKeypress(keyPress("enter")); cmd != nil {
		t.Fatalf("enter into editor should not emit a message yet, got %T", cmd)
	}
	if s.stage != 1 {
		t.Fatalf("stage = %d, want 1 (editor)", s.stage)
	}
	if s.target != targetAgentWrite {
		t.Fatalf("target = %v, want targetAgentWrite", s.target)
	}
}

// Typing digits fills the focused field; tab switches to the output field;
// enter persists via setting.UpdateTokenLimitFor and emits TokenLimitSavedMsg.
func TestTokenLimitSelectorCommitPersistsAndCloses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setting.ResetDefaultSettings()
	t.Cleanup(setting.ResetDefaultSettings)

	s := NewTokenLimitSelector()
	s.EnterSelect(80, 24)
	// Pick "Sub-agent" (second row).
	s.HandleKeypress(keyPress("down"))
	s.HandleKeypress(keyPress("enter"))
	if s.target != targetAgent {
		t.Fatalf("target = %v, want targetAgent", s.target)
	}

	// Type "48000" into input, tab, "4000" into output, enter.
	for _, r := range "48000" {
		s.HandleKeypress(keyPress(string(r)))
	}
	s.HandleKeypress(keyPress("tab"))
	for _, r := range "4000" {
		s.HandleKeypress(keyPress(string(r)))
	}
	var gotMsg tea.Msg
	cmd := s.HandleKeypress(keyPress("enter"))
	if cmd == nil {
		t.Fatal("enter should emit a save message")
	}
	gotMsg = cmd()

	msg, ok := gotMsg.(TokenLimitSavedMsg)
	if !ok {
		t.Fatalf("got %T, want TokenLimitSavedMsg", gotMsg)
	}
	if msg.Target != "Sub-agent" || msg.Input != 48000 || msg.Output != 4000 {
		t.Fatalf("msg = %+v, want Sub-agent in=48000 out=4000", msg)
	}
	if s.IsActive() {
		t.Fatal("selector should close after save")
	}

	// The override landed in the global settings (user-level write).
	d, err := setting.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if d.AgentTokenLimit.InputTokenLimit != 48000 || d.AgentTokenLimit.OutputTokenLimit != 4000 {
		t.Fatalf("persisted AgentTokenLimit = %+v, want in=48000 out=4000", d.AgentTokenLimit)
	}
	if d.MainTokenLimit != (setting.TokenLimitOverride{}) {
		t.Fatalf("MainTokenLimit should stay unset, got %+v", d.MainTokenLimit)
	}
}

// Zeroing both fields clears the override for the role; a non-numeric or
// negative entry is rejected in place.
func TestTokenLimitSelectorRejectsInvalidAndClears(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setting.ResetDefaultSettings()
	t.Cleanup(setting.ResetDefaultSettings)

	s := NewTokenLimitSelector()
	s.EnterSelect(80, 24)
	s.HandleKeypress(keyPress("enter")) // Main agent

	// Non-digit text is ignored (stays empty → parse fails, error shown).
	for _, r := range "abc" {
		s.HandleKeypress(keyPress(string(r)))
	}
	if cmd := s.HandleKeypress(keyPress("enter")); cmd != nil {
		t.Fatal("invalid numbers must not emit a save message")
	}
	if s.errorText == "" {
		t.Fatal("expected an inline error for non-numeric input")
	}

	// Entering 0 0 clears and closes.
	s.value[0] = "0"
	s.value[1] = "0"
	cmd := s.HandleKeypress(keyPress("enter"))
	if cmd == nil {
		t.Fatal("zero-zero should save as a clear")
	}
	if msg := cmd().(TokenLimitSavedMsg); msg.Input != 0 || msg.Output != 0 {
		t.Fatalf("clear msg = %+v, want zeros", msg)
	}

	d, err := setting.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if d.MainTokenLimit != (setting.TokenLimitOverride{}) {
		t.Fatalf("MainTokenLimit should be cleared, got %+v", d.MainTokenLimit)
	}
}

// The status-bar happy path: a saved budget formats into the status line.
func TestTokenLimitSavedStatusFormat(t *testing.T) {
	saved := TokenLimitSavedMsg{Target: "Main agent", Input: 200000, Output: 16000}
	status := "Token limits saved: " + saved.Target + " — in " +
		kit.FormatTokenCount(saved.Input) + " · out " + kit.FormatTokenCount(saved.Output)
	if !strings.Contains(status, "in 200.0k") || !strings.Contains(status, "out 16.0k") {
		t.Fatalf("status = %q, want formatted counts", status)
	}
}
