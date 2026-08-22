package llm

import (
	"testing"

	"github.com/boytegar/packboy-builder/internal/setting"
)

// The settings.json tokenLimit override is the highest-priority source for the
// context window: it beats the per-model tokenLimits in providers.json and the
// PCB_INPUT_LIMIT env var, so one value shapes the auto-compaction trigger and
// the status bar for every model.
func TestEffectiveInputLimitSettingsOverrideWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(InputLimitEnvVar, "272000")
	setting.SetDefaultSettings(setting.New(&setting.Data{TokenLimit: 200000}))
	t.Cleanup(setting.ResetDefaultSettings)

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTokenLimit("m", 400000, 8000); err != nil {
		t.Fatalf("SetTokenLimit: %v", err)
	}

	if got := store.EffectiveInputLimit(OpenAI, AuthAPIKey, "m"); got != 200000 {
		t.Fatalf("EffectiveInputLimit() = %d, want the 200000 settings override (beats env 272000 and per-model 400000)", got)
	}
}

// With no settings override, the previous priority order is intact: env beats
// per-model, per-model beats cache.
func TestEffectiveInputLimitEnvBeatsPerModelWhenNoSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(InputLimitEnvVar, "272000")
	setting.ResetDefaultSettings()
	t.Cleanup(setting.ResetDefaultSettings)

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTokenLimit("m", 400000, 8000); err != nil {
		t.Fatalf("SetTokenLimit: %v", err)
	}

	if got := store.EffectiveInputLimit(OpenAI, AuthAPIKey, "m"); got != 272000 {
		t.Fatalf("EffectiveInputLimit() = %d, want the 272000 env override", got)
	}
}

// A zero/absent settings override does not clobber the rest of the chain — it
// must not resolve to 0 when another source knows the window.
func TestEffectiveInputLimitZeroSettingsFallsThrough(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(InputLimitEnvVar, "")
	setting.ResetDefaultSettings()
	t.Cleanup(setting.ResetDefaultSettings)

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTokenLimit("m", 200000, 8000); err != nil {
		t.Fatalf("SetTokenLimit: %v", err)
	}

	if got := store.EffectiveInputLimit(OpenAI, AuthAPIKey, "m"); got != 200000 {
		t.Fatalf("EffectiveInputLimit() = %d, want the 200000 per-model limit", got)
	}
}

// The role-scoped /tokenlimit override wins for its own role: a main budget
// must not leak into a subagent's window, and the global tokenLimit still
// applies to a role without its own override.
func TestEffectiveInputLimitForRoleScopedOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(InputLimitEnvVar, "")
	setting.SetDefaultSettings(setting.New(&setting.Data{
		MainTokenLimit:  setting.TokenLimitOverride{InputTokenLimit: 120000, OutputTokenLimit: 6000},
		AgentTokenLimit: setting.TokenLimitOverride{InputTokenLimit: 48000, OutputTokenLimit: 4000},
	}))
	t.Cleanup(setting.ResetDefaultSettings)

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if got := store.EffectiveInputLimitFor(TokenRoleMain, OpenAI, AuthAPIKey, "m"); got != 120000 {
		t.Fatalf("EffectiveInputLimitFor(main) = %d, want the 120000 main override", got)
	}
	if got := store.EffectiveInputLimitFor(TokenRoleAgent, OpenAI, AuthAPIKey, "m"); got != 48000 {
		t.Fatalf("EffectiveInputLimitFor(agent) = %d, want the 48000 agent override", got)
	}
	if got := store.EffectiveInputLimit(OpenAI, AuthAPIKey, "m"); got != 0 {
		t.Fatalf("EffectiveInputLimit() = %d, want 0 (role overrides must not leak into the un-scoped resolver)", got)
	}
}

// A role without its own override falls through to the global tokenLimit; the
// role override takes priority over the global one when both are set.
func TestEffectiveInputLimitForRoleFallsThroughToGlobal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(InputLimitEnvVar, "")
	setting.SetDefaultSettings(setting.New(&setting.Data{
		TokenLimit:      200000,
		AgentTokenLimit: setting.TokenLimitOverride{InputTokenLimit: 48000},
	}))
	t.Cleanup(setting.ResetDefaultSettings)

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if got := store.EffectiveInputLimitFor(TokenRoleMain, OpenAI, AuthAPIKey, "m"); got != 200000 {
		t.Fatalf("EffectiveInputLimitFor(main) = %d, want the 200000 global tokenLimit (no main override)", got)
	}
	if got := store.EffectiveInputLimitFor(TokenRoleAgent, OpenAI, AuthAPIKey, "m"); got != 48000 {
		t.Fatalf("EffectiveInputLimitFor(agent) = %d, want the 48000 agent override (beats global 200000)", got)
	}
}
