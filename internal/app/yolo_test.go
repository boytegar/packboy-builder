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
	if m.env.OperationMode != setting.ModeBypassPermissions {
		t.Fatalf("mode = %v, want bypass", m.env.OperationMode)
	}
	if m.env.SessionPermissions.Snapshot().Mode != setting.ModeBypassPermissions {
		t.Fatalf("session mode not mirrored to bypass")
	}
	if !strings.Contains(notice, "on") {
		t.Errorf("notice = %q, want on", notice)
	}
}

func TestApplyYoloModeToggleOff(t *testing.T) {
	m := yoloModel(t, true, setting.ModeBypassPermissions)
	m.env.ApplyModePermissions(m.env.CWD)
	notice := m.applyYoloMode(nil)
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("mode = %v, want normal", m.env.OperationMode)
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
	if m.env.OperationMode != setting.ModeBypassPermissions {
		t.Fatalf("after on: mode = %v", m.env.OperationMode)
	}
	if notice := m.applyYoloMode(boolPtr(false)); !strings.Contains(notice, "off") {
		t.Errorf("off notice = %q", notice)
	}
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("after off: mode = %v", m.env.OperationMode)
	}
}

func TestApplyYoloModeLockedOut(t *testing.T) {
	m := yoloModel(t, false, setting.ModeNormal)
	notice := m.applyYoloMode(boolPtr(true))
	if m.env.OperationMode != setting.ModeNormal {
		t.Fatalf("locked enable flipped mode to %v", m.env.OperationMode)
	}
	if !strings.Contains(notice, "locked out") {
		t.Errorf("notice = %q, want locked-out text", notice)
	}
}

func TestApplyYoloModeIdempotent(t *testing.T) {
	m := yoloModel(t, true, setting.ModeBypassPermissions)
	m.env.ApplyModePermissions(m.env.CWD)
	notice := m.applyYoloMode(boolPtr(true))
	if !strings.Contains(notice, "already on") {
		t.Errorf("notice = %q, want already on", notice)
	}
}
