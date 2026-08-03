package app

import (
	"context"
	"testing"

	"github.com/boytegar/packboy-builder/internal/hook"
	"github.com/boytegar/packboy-builder/internal/setting"
)

func TestNewBaseModelSyncsRestoredOperationModeToHooks(t *testing.T) {
	cwd := t.TempDir()
	settings := setting.NewData()
	settings.LastOperationMode = "auto-accept"

	engine := hook.NewEngine(setting.NewData(), "", cwd, "")
	var hookMode string
	engine.AddSessionFunctionHook(hook.PreToolUse, "", hook.FunctionHook{
		Callback: func(_ context.Context, input hook.HookInput) (hook.HookOutput, error) {
			hookMode = input.PermissionMode
			return hook.HookOutput{}, nil
		},
	})

	environment := env{SessionPermissions: setting.NewSessionPermissions()}
	applyStartupSettings(&environment, settings, cwd, true, engine)
	engine.Execute(context.Background(), hook.PreToolUse, hook.HookInput{ToolName: "Bash"})

	if environment.OperationMode != setting.ModeAutoAccept {
		t.Fatalf("OperationMode = %v, want %v", environment.OperationMode, setting.ModeAutoAccept)
	}
	if hookMode != "auto" {
		t.Fatalf("hook permission mode = %q, want %q", hookMode, "auto")
	}
}

// TestApplyStartupSettingsRestoresPersistedBypass confirms the persisted
// bypass-permissions toggle is restored into the session on startup, so
// /yolo survives restarts independent of the operation mode.
func TestApplyStartupSettingsRestoresPersistedBypass(t *testing.T) {
	cwd := t.TempDir()
	settings := setting.NewData()
	on := true
	settings.BypassEnabled = &on

	engine := hook.NewEngine(setting.NewData(), "", cwd, "")

	environment := env{SessionPermissions: setting.NewSessionPermissions()}
	applyStartupSettings(&environment, settings, cwd, true, engine)

	if !environment.SessionPermissions.IsBypass() {
		t.Fatalf("persisted bypass not restored: IsBypass=false, want true")
	}

	// Off must restore off.
	settings2 := setting.NewData()
	off := false
	settings2.BypassEnabled = &off
	env2 := env{SessionPermissions: setting.NewSessionPermissions()}
	applyStartupSettings(&env2, settings2, cwd, true, engine)
	if env2.SessionPermissions.IsBypass() {
		t.Fatalf("persisted bypass-off not respected: IsBypass=true, want false")
	}

	// Nil (unset) must leave bypass off.
	settings3 := setting.NewData()
	env3 := env{SessionPermissions: setting.NewSessionPermissions()}
	applyStartupSettings(&env3, settings3, cwd, true, engine)
	if env3.SessionPermissions.IsBypass() {
		t.Fatalf("unset bypass defaulted on: IsBypass=true, want false")
	}
}
