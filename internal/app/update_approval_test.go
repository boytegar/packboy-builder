package app

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/confdir"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/tool/perm"
)

func TestPermDecisionRecordSnapshotsInput(t *testing.T) {
	input := map[string]any{"prompt": "inspect the repository"}
	req := &conv.PermGateRequest{
		RequestID: "permission-1",
		ToolName:  "Agent",
		Input:     input,
	}

	record := permDecisionRecord(req, permissionDecision{
		Approved: true,
		Request:  &perm.PermissionRequest{ToolName: "Agent"},
	}, "user approved", "normal")

	// Agent execution decorates this same input map after the permission gate
	// opens. The permission record must already own serialized bytes by then.
	input["_onActivity"] = func() {}
	input["prompt"] = "changed after approval"

	if got, want := string(record.Input), `{"prompt":"inspect the repository"}`; got != want {
		t.Fatalf("permission input snapshot = %s, want %s", got, want)
	}
}

func TestPersistentAllowRulesPreferSuggestions(t *testing.T) {
	ui := &perm.PermissionRequest{
		ToolName:       "Bash",
		SuggestedRules: []string{"Bash(git:commit *)", "Bash(npm:test *)"},
	}
	got := persistentAllowRules(ui, "Bash", map[string]any{"command": "git commit -m x"})
	want := []string{"Bash(git:commit *)", "Bash(npm:test *)"}
	if !slices.Equal(got, want) {
		t.Fatalf("persistentAllowRules = %v, want %v", got, want)
	}
}

func TestPersistentAllowRulesFallbackBuildRule(t *testing.T) {
	input := map[string]any{"command": "go test ./..."}
	got := persistentAllowRules(nil, "Bash", input)
	want := setting.BuildRule("Bash", input)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("persistentAllowRules fallback = %v, want [%q]", got, want)
	}
}

func TestApplyPersistentAllowGrantsSessionAndDisk(t *testing.T) {
	cwd := t.TempDir()
	m := &model{
		env: env{
			CWD:                cwd,
			SessionPermissions: setting.NewSessionPermissions(),
		},
		services: services{
			Setting: setting.New(setting.NewData()),
		},
	}
	rule := "Bash(go:test *)"
	m.applyPersistentAllow(&perm.PermissionRequest{
		ToolName:       "Bash",
		SuggestedRules: []string{rule},
	}, "Bash", map[string]any{"command": "go test ./..."})

	snap := m.env.SessionPermissions.Snapshot()
	if !snap.AllowedPatterns[rule] {
		t.Fatalf("session pattern %q not granted: %#v", rule, snap.AllowedPatterns)
	}

	// Session grant alone must short-circuit the next matching call even
	// before disk reload — confirmation checks still sit above allow rules,
	// so the session pattern is what stops re-prompts mid-turn.
	d := setting.NewData().HasPermissionToUseTool(
		"Bash",
		map[string]any{"command": "go test ./internal/..."},
		snap,
	)
	if d.Behavior != perm.Permit {
		t.Fatalf("session grant decision = %+v, want Permit (reason %q)", d, d.Reason)
	}

	path := filepath.Join(confdir.Dir(cwd), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read project settings at %s: %v", path, err)
	}
	if !bytes.Contains(data, []byte(rule)) {
		t.Fatalf("settings.json missing rule %q:\n%s", rule, data)
	}

	if err := m.services.Setting.Reload(cwd); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	live := m.services.Setting.HasPermissionToUseTool(
		"Bash",
		map[string]any{"command": "go test ./..."},
		nil,
	)
	if live.Behavior != perm.Permit {
		t.Fatalf("reloaded allow rule decision = %+v, want Permit", live)
	}
}

func TestPreparePermissionRequestFillsSuggestedRules(t *testing.T) {
	m := &model{env: env{CWD: t.TempDir()}}
	req := &conv.PermGateRequest{
		ToolName: "Bash",
		Input:    map[string]any{"command": "git commit -m fix"},
	}
	rich := m.preparePermissionRequest(req)
	if rich == nil || len(rich.SuggestedRules) == 0 {
		t.Fatalf("SuggestedRules empty: %#v", rich)
	}
	if rich.SuggestedRules[0] != "Bash(git:commit *)" {
		t.Fatalf("SuggestedRules[0] = %q, want Bash(git:commit *)", rich.SuggestedRules[0])
	}
}
