package app

import (
	"testing"

	"github.com/boytegar/packboy-builder/internal/agent"
	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/hook"
	"github.com/boytegar/packboy-builder/internal/session"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/todo"
)

// modelForNewSession builds a model with just enough wiring for applyNewSession:
// a session service that carries an ID, a tracker with a storage dir, and a
// settings store for the autopilot default.
func modelForNewSession(t *testing.T) *model {
	t.Helper()
	data := setting.NewData()
	m := &model{
		conv: conv.NewModel(80),
		env: env{
			SessionName: "custom-name",
		},
		services: services{
			Agent:   &agent.Session{},
			Hook:    hook.NewEngine(data, "test-session", t.TempDir(), ""),
			Setting: setting.New(data),
			Session: &session.Setup{},
			Tracker: todo.NewStore(),
		},
	}
	m.services.Session.SetID("session-123")
	_ = m.services.Tracker.SetStorageDir("/tmp/session-123-tasks")
	m.chat = chatViewer(80, 20)
	// Seed the committed-block cache like a conversation that has scrolled.
	m.chat.rebuildCache([]string{"stale committed block\n"})
	return m
}

func TestApplyNewSessionBlanksIdentityAndReArmsSplash(t *testing.T) {
	m := modelForNewSession(t)

	m.applyNewSession()

	if got := m.services.Session.ID(); got != "" {
		t.Fatalf("session ID = %q, want empty after new session", got)
	}
	if m.env.SessionName != "" {
		t.Fatalf("env.SessionName = %q, want empty after new session", m.env.SessionName)
	}
	if got := m.services.Tracker.GetStorageDir(); got != "" {
		t.Fatalf("tracker storage dir = %q, want empty after new session", got)
	}
	if !m.welcomePending {
		t.Fatal("welcomePending = false, want true so the splash re-arms")
	}
	if m.statusPanelScrollX != 0 {
		t.Fatalf("statusPanelScrollX = %d, want 0", m.statusPanelScrollX)
	}
	// The committed-block cache must be dropped: the conversation was cleared,
	// and a stale cache would keep the old text rendering (or, if the frame
	// compares equal, let the renderer skip the redraw after the /new erase —
	// the blank-screen bug).
	if len(m.chat.renderedBlocks) != 0 {
		t.Fatalf("viewport cache = %d blocks after /new, want 0 (stale committed content must not survive)", len(m.chat.renderedBlocks))
	}
	if !m.chat.dirty {
		t.Fatal("viewport not marked dirty after /new — the next frame would skip the redraw")
	}
}
