package subagent

import "testing"

func TestBuiltinAgentsRegistered(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	configs := builtinAgentConfigs()
	if len(configs) == 0 {
		t.Fatal("builtinAgentConfigs() returned no agents")
	}

	for _, expected := range configs {
		got, ok := r.Get(expected.Name)
		if !ok {
			t.Errorf("builtin agent %q not registered", expected.Name)
			continue
		}
		if got.Description != expected.Description {
			t.Errorf("agent %q description mismatch: got %q want %q", expected.Name, got.Description, expected.Description)
		}
		if got.PermissionMode != expected.PermissionMode {
			t.Errorf("agent %q mode mismatch: got %q want %q", expected.Name, got.PermissionMode, expected.PermissionMode)
		}
		if got.SystemPrompt == "" {
			t.Errorf("agent %q must have a system prompt", expected.Name)
		}
		if got.Source != "builtin" {
			t.Errorf("agent %q source mismatch: got %q want %q", expected.Name, got.Source, "builtin")
		}
	}
}

func TestBuiltinResearcherIsExploreMode(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	config, ok := r.Get("researcher")
	if !ok {
		t.Fatal("researcher agent not registered")
	}
	if config.PermissionMode != PermissionExplore {
		t.Errorf("researcher mode = %q, want %q", config.PermissionMode, PermissionExplore)
	}
	if config.MaxSteps != 0 {
		t.Errorf("researcher max-steps = %d, want 0 (unlimited)", config.MaxSteps)
	}
	if config.Model != "inherit" {
		t.Errorf("researcher model = %q, want inherit", config.Model)
	}
}

func TestBuiltinAgentsOverriddenByFile(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	// A user/project file with the same name must override the built-in.
	override := &AgentConfig{
		Name:           "researcher",
		Description:    "custom override",
		PermissionMode: PermissionDefault,
		Model:          "inherit",
	}
	r.Register(override)

	got, ok := r.Get("researcher")
	if !ok {
		t.Fatal("researcher not found after override")
	}
	if got.Description != "custom override" {
		t.Errorf("override did not take effect: got %q", got.Description)
	}
}
