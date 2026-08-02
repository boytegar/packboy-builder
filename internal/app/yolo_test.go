package app

import (
	"strings"
	"testing"

	"github.com/boytegar/packboy-builder/internal/hook"
	"github.com/boytegar/packboy-builder/internal/setting"
)

func yoloModel(t *testing.T, allowBypass bool, mode setting.OperationMode) *model {
	t.Helper()
	data := setting.NewData()
	if !allowBypass {
		off := false
		data.AllowBypass = &off
	}
	return &model{
		env: env{
			CWD:                t.TempDir(),
			OperationMode:      mode,
			SessionPermissions: setting.NewSessionPermissions(),
		},
		services: services{
			Setting: setting.New(data),
			Hook:    hook.NewEngine(data, "test-session", t.TempDir(), ""),
		},
	}
}

func boolPtr(v bool) *bool { return &v }

func TestApplyYoloModeToggleOn(t *testing.T) {
	m := yoloModel(t, true, setting.ModeNormal)
	notice := m.applyYoloMode(nil)
	// Bypass is now an orthogonal flag — the operation mode must not change.
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("mode = %v, want unchanged (Normal)", m.env.OperationMode)
	}
	if !m.env.SessionPermissions.IsBypass() {
		t.Fatalf("session bypass flag not set")
	}
	if !strings.Contains(notice, "on") {
		t.Errorf("notice = %q, want on", notice)
	}
}

func TestApplyYoloModeToggleOff(t *testing.T) {
	m := yoloModel(t, true, setting.ModeNormal)
	m.env.SessionPermissions.SetBypass(true)
	notice := m.applyYoloMode(nil)
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("mode = %v, want unchanged (Normal)", m.env.OperationMode)
	}
	if m.env.SessionPermissions.IsBypass() {
		t.Fatalf("bypass flag still on after toggle off")
	}
	if !strings.Contains(notice, "off") {
		t.Errorf("notice = %q, want off", notice)
	}
}

func TestApplyYoloModeOnOffExplicit(t *testing.T) {
	m := yoloModel(t, true, setting.ModeNormal)
	if notice := m.applyYoloMode(boolPtr(true)); !strings.Contains(notice, "on") {
		t.Errorf("on notice = %q", notice)
	}
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("after on: mode = %v, want Normal (bypass is orthogonal)", m.env.OperationMode)
	}
	if !m.env.SessionPermissions.IsBypass() {
		t.Fatalf("after on: bypass flag not set")
	}
	if notice := m.applyYoloMode(boolPtr(false)); !strings.Contains(notice, "off") {
		t.Errorf("off notice = %q", notice)
	}
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("after off: mode = %v, want Normal", m.env.OperationMode)
	}
	if m.env.SessionPermissions.IsBypass() {
		t.Fatalf("after off: bypass flag still on")
	}
}

func TestApplyYoloModeLockedOut(t *testing.T) {
	m := yoloModel(t, false, setting.ModeNormal)
	notice := m.applyYoloMode(boolPtr(true))
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("locked enable flipped mode to %v", m.env.OperationMode)
	}
	if m.env.SessionPermissions.IsBypass() {
		t.Fatalf("locked enable set bypass flag")
	}
	if !strings.Contains(notice, "locked out") {
		t.Errorf("notice = %q, want locked-out text", notice)
	}
}

func TestApplyYoloModeIdempotent(t *testing.T) {
	m := yoloModel(t, true, setting.ModeNormal)
	m.env.SessionPermissions.SetBypass(true)
	notice := m.applyYoloMode(boolPtr(true))
	if !strings.Contains(notice, "already on") {
		t.Errorf("notice = %q, want already on", notice)
	}
}

// TestApplyYoloMode_PreservesMode confirms bypass is orthogonal: toggling /yolo
// on while in chat or agent mode must not change the active mode.
func TestApplyYoloMode_PreservesChatMode(t *testing.T) {
	m := yoloModel(t, true, setting.ModeReadOnly)
	notice := m.applyYoloMode(boolPtr(true))
	if m.env.OperationMode != setting.ModeReadOnly {
		t.Fatalf("mode = %v, want ReadOnly (bypass must not change mode)", m.env.OperationMode)
	}
	if !m.env.SessionPermissions.IsBypass() {
		t.Fatalf("bypass flag not set")
	}
	if !strings.Contains(notice, "chat") {
		t.Errorf("notice = %q, want to mention active mode", notice)
	}
}
