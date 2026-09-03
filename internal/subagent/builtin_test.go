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

func TestBuiltinPlannerRegistered(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	config, ok := r.Get("planner")
	if !ok {
		t.Fatal("planner agent not registered")
	}
	if config.PermissionMode != PermissionExplore {
		t.Errorf("planner mode = %q, want %q", config.PermissionMode, PermissionExplore)
	}
	if config.MaxSteps != 30 {
		t.Errorf("planner max-steps = %d, want 30", config.MaxSteps)
	}
	if config.Model != "inherit" {
		t.Errorf("planner model = %q, want inherit", config.Model)
	}
	if config.SystemPrompt == "" {
		t.Error("planner must have a system prompt")
	}
}

func TestBuiltinImplementerRegistered(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	config, ok := r.Get("implementer")
	if !ok {
		t.Fatal("implementer agent not registered")
	}
	if config.PermissionMode != PermissionAcceptEdits {
		t.Errorf("implementer mode = %q, want %q", config.PermissionMode, PermissionAcceptEdits)
	}
	if !config.AllowWrite {
		t.Error("implementer must have allow_write = true")
	}
	if config.MaxSteps != 50 {
		t.Errorf("implementer max-steps = %d, want 50", config.MaxSteps)
	}
	if config.Model != "inherit" {
		t.Errorf("implementer model = %q, want inherit", config.Model)
	}
	if config.SystemPrompt == "" {
		t.Error("implementer must have a system prompt")
	}
}

func TestBuiltinTesterRegistered(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	config, ok := r.Get("tester")
	if !ok {
		t.Fatal("tester agent not registered")
	}
	if config.PermissionMode != PermissionAcceptEdits {
		t.Errorf("tester mode = %q, want %q", config.PermissionMode, PermissionAcceptEdits)
	}
	if !config.AllowWrite {
		t.Error("tester must have allow_write = true")
	}
	if config.MaxSteps != 40 {
		t.Errorf("tester max-steps = %d, want 40", config.MaxSteps)
	}
	if config.Model != "inherit" {
		t.Errorf("tester model = %q, want inherit", config.Model)
	}
	if config.SystemPrompt == "" {
		t.Error("tester must have a system prompt")
	}
}

func TestBuiltinWorkerRegistered(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	config, ok := r.Get("worker")
	if !ok {
		t.Fatal("worker agent not registered")
	}
	if config.PermissionMode != PermissionAcceptEdits {
		t.Errorf("worker mode = %q, want %q", config.PermissionMode, PermissionAcceptEdits)
	}
	if !config.AllowWrite {
		t.Error("worker must have allow_write = true")
	}
	if !config.GeneralPurpose {
		t.Error("worker must be general-purpose")
	}
	if config.Complexity != "medium" {
		t.Errorf("worker complexity = %q, want medium", config.Complexity)
	}
	if config.SystemPrompt == "" {
		t.Error("worker must have a system prompt")
	}
	if config.Model != "inherit" {
		t.Errorf("worker model = %q, want inherit", config.Model)
	}
	if config.Source != "builtin" {
		t.Errorf("worker source = %q, want builtin", config.Source)
	}
}

func TestBuiltinExplorerRegistered(t *testing.T) {
	r := NewRegistry()
	registerBuiltinAgents(r)

	config, ok := r.Get("explorer")
	if !ok {
		t.Fatal("explorer agent not registered")
	}
	if config.PermissionMode != PermissionExplore {
		t.Errorf("explorer mode = %q, want %q", config.PermissionMode, PermissionExplore)
	}
	if !config.GeneralPurpose {
		t.Error("explorer must be general-purpose")
	}
	if config.Complexity != "light" {
		t.Errorf("explorer complexity = %q, want light", config.Complexity)
	}
	if config.SystemPrompt == "" {
		t.Error("explorer must have a system prompt")
	}
	if config.Model != "inherit" {
		t.Errorf("explorer model = %q, want inherit", config.Model)
	}
	if config.Source != "builtin" {
		t.Errorf("explorer source = %q, want builtin", config.Source)
	}
}

func TestBuiltinAgentsCount(t *testing.T) {
	configs := builtinAgentConfigs()
	if len(configs) != 6 {
		t.Errorf("expected 6 builtin agents (researcher, planner, implementer, tester, worker, explorer), got %d", len(configs))
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
