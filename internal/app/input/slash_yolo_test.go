package input

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/setting"
)

func runYolo(t *testing.T, args string, settings *setting.Settings) (string, tea.Msg) {
	t.Helper()
	c := NewSlashCommandController(SlashCommandEnv{Setting: settings})
	notice, cmd, err := c.handleYoloCommand(context.Background(), args)
	if err != nil {
		t.Fatalf("handleYoloCommand(%q) err = %v", args, err)
	}
	if cmd == nil {
		return notice, nil
	}
	return notice, cmd()
}

func TestYoloCommandToggle(t *testing.T) {
	notice, msg := runYolo(t, "", setting.New(setting.NewData()))
	if notice != "" {
		t.Errorf("toggle should speak through the app, not a notice; got %q", notice)
	}
	yolo, ok := msg.(YoloMsg)
	if !ok {
		t.Fatalf("got %T, want YoloMsg", msg)
	}
	if yolo.Enable != nil {
		t.Errorf("bare /yolo Enable = %v, want nil (toggle)", *yolo.Enable)
	}
}

func TestYoloCommandOnOff(t *testing.T) {
	for _, tt := range []struct {
		arg  string
		want bool
	}{
		{"on", true},
		{"OFF", false},
		{"enable", true},
		{"disable", false},
		{"1", true},
		{"0", false},
	} {
		_, msg := runYolo(t, tt.arg, setting.New(setting.NewData()))
		yolo, ok := msg.(YoloMsg)
		if !ok {
			t.Fatalf("/yolo %s: got %T, want YoloMsg", tt.arg, msg)
		}
		if yolo.Enable == nil || *yolo.Enable != tt.want {
			t.Errorf("/yolo %s: Enable = %v, want %v", tt.arg, yolo.Enable, tt.want)
		}
	}
}

func TestYoloCommandUsage(t *testing.T) {
	notice, msg := runYolo(t, "maybe", nil)
	if msg != nil {
		t.Errorf("bad args should not emit a mode change; got %T", msg)
	}
	if !strings.Contains(notice, "Usage: /yolo") {
		t.Errorf("notice = %q, want usage text", notice)
	}
}

func TestYoloCommandLockedOut(t *testing.T) {
	data := setting.NewData()
	off := false
	data.AllowBypass = &off
	notice, msg := runYolo(t, "on", setting.New(data))
	if msg != nil {
		t.Errorf("locked-out enable should not emit YoloMsg; got %T", msg)
	}
	if !strings.Contains(notice, "locked out") {
		t.Errorf("notice = %q, want locked-out text", notice)
	}
}

func TestYoloCommandOffWhenLockedStillEmits(t *testing.T) {
	// Turning off must still work even when allowBypass is false — the user
	// may already be in bypass from a prior session/settings default.
	data := setting.NewData()
	off := false
	data.AllowBypass = &off
	_, msg := runYolo(t, "off", setting.New(data))
	yolo, ok := msg.(YoloMsg)
	if !ok || yolo.Enable == nil || *yolo.Enable {
		t.Fatalf("/yolo off under lock: got %#v, want Enable=false", msg)
	}
}
