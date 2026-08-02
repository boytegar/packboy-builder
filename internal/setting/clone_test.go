package setting

import "testing"

// TestClonePreservesAllScalarFields guards against Clone() drift that would
// silently revert a setting to its default at startup: every scalar field on
// Data must round-trip through Clone. New fields should be added here at the
// same time they are added to Clone().
func TestClonePreservesAllScalarFields(t *testing.T) {
	yes := true
	src := &Data{
		Model:          "claude-opus-4-7",
		TokenLimit:     200000,
		Theme:          "dark",
		SearchProvider: "exa",
		AllowBypass:    &yes,
		Persona:        "ml-researcher",
		SkillDirs:      []string{"/mnt/shared/skills", "~/team-skills"},
		SubagentDefaultModel: "haiku",
		SelfLearn: SelfLearnSettings{
			Memory:   SelfLearnMemory{Enabled: true, MaxKB: 15},
			Skills:   SelfLearnSkills{DenyCreate: true},
			Strategy: "custom",
		},
	}

	dst := src.Clone()

	if dst.Model != src.Model {
		t.Errorf("Model: got %q, want %q", dst.Model, src.Model)
	}
	if dst.TokenLimit != src.TokenLimit {
		t.Errorf("TokenLimit: got %d, want %d", dst.TokenLimit, src.TokenLimit)
	}
	if dst.Theme != src.Theme {
		t.Errorf("Theme: got %q, want %q", dst.Theme, src.Theme)
	}
	if dst.SearchProvider != src.SearchProvider {
		t.Errorf("SearchProvider: got %q, want %q", dst.SearchProvider, src.SearchProvider)
	}
	if dst.AllowBypass == nil || *dst.AllowBypass != *src.AllowBypass {
		t.Errorf("AllowBypass: got %v, want %v", dst.AllowBypass, src.AllowBypass)
	}
	if dst.Persona != src.Persona {
		t.Errorf("Persona: got %q, want %q", dst.Persona, src.Persona)
	}
	if dst.SubagentDefaultModel != src.SubagentDefaultModel {
		t.Errorf("SubagentDefaultModel: got %q, want %q", dst.SubagentDefaultModel, src.SubagentDefaultModel)
	}
	// SkillDirs must round-trip and be an independent copy so mutating dst
	// cannot bleed back into src.
	if len(dst.SkillDirs) != len(src.SkillDirs) {
		t.Errorf("SkillDirs len: got %d, want %d", len(dst.SkillDirs), len(src.SkillDirs))
	} else {
		for i := range src.SkillDirs {
			if dst.SkillDirs[i] != src.SkillDirs[i] {
				t.Errorf("SkillDirs[%d]: got %q, want %q", i, dst.SkillDirs[i], src.SkillDirs[i])
			}
		}
		dst.SkillDirs[0] = "/mutated"
		if src.SkillDirs[0] == "/mutated" {
			t.Error("SkillDirs is shared, not a copy — Clone must deep-copy the slice")
		}
	}
	// SelfLearn is value-typed; the whole struct (incl. nested Memory /
	// Skills) must survive. Skipping this row caused /config to silently
	// show stale defaults until the bug was caught.
	if dst.SelfLearn != src.SelfLearn {
		t.Errorf("SelfLearn: got %+v, want %+v", dst.SelfLearn, src.SelfLearn)
	}
}

// TestMergeSettingsPreservesSelfLearn guards the merger gap that left the
// entire L1 feature unreachable from settings.json: mergeSettings used to
// drop the SelfLearn field on every load and every save merge.
// TestMergeSettingsMergesSubagentModels guards the per-key merge for the
// subagentModels map: overlay wins per agent, base survives an overlay that
// doesn't mention it.
func TestMergeSettingsMergesSubagentModels(t *testing.T) {
	base := &Data{SubagentModels: map[string]string{"coder": "haiku", "reviewer": "sonnet"}}
	overlay := &Data{SubagentModels: map[string]string{"coder": "opus"}}

	got := mergeSettings(base, overlay)
	if got.SubagentModels["coder"] != "opus" {
		t.Errorf("overlay coder: got %q, want opus", got.SubagentModels["coder"])
	}
	if got.SubagentModels["reviewer"] != "sonnet" {
		t.Errorf("base reviewer: got %q, want sonnet", got.SubagentModels["reviewer"])
	}

	// Base survives an empty overlay.
	got = mergeSettings(&Data{SubagentModels: map[string]string{"coder": "haiku"}}, &Data{})
	if got.SubagentModels["coder"] != "haiku" {
		t.Errorf("base-only coder: got %q, want haiku", got.SubagentModels["coder"])
	}
}

// TestCloneSubagentModels guards the deep clone of the subagentModels map so a
// mutation on the clone never leaks back into the source.
func TestCloneSubagentModels(t *testing.T) {
	src := &Data{SubagentModels: map[string]string{"coder": "haiku"}}
	clone := src.Clone()
	clone.SubagentModels["coder"] = "opus"
	if src.SubagentModels["coder"] != "haiku" {
		t.Errorf("clone leaked: src coder = %q, want haiku", src.SubagentModels["coder"])
	}
}

// TestMergeSettingsCoalescesSubagentDefaultModel guards the coalesce merge for
// the global default: overlay's non-empty value wins; base survives an empty
// overlay.
func TestMergeSettingsCoalescesSubagentDefaultModel(t *testing.T) {
	base := &Data{SubagentDefaultModel: "haiku"}
	overlay := &Data{SubagentDefaultModel: "opus"}

	got := mergeSettings(base, overlay)
	if got.SubagentDefaultModel != "opus" {
		t.Errorf("overlay default: got %q, want opus", got.SubagentDefaultModel)
	}

	got = mergeSettings(&Data{SubagentDefaultModel: "haiku"}, &Data{})
	if got.SubagentDefaultModel != "haiku" {
		t.Errorf("base-only default: got %q, want haiku", got.SubagentDefaultModel)
	}
}

// TestCloneSubagentDefaultModel guards the scalar clone of the global default.
func TestCloneSubagentDefaultModel(t *testing.T) {
	src := &Data{SubagentDefaultModel: "haiku"}
	clone := src.Clone()
	clone.SubagentDefaultModel = "opus"
	if src.SubagentDefaultModel != "haiku" {
		t.Errorf("clone leaked: src default = %q, want haiku", src.SubagentDefaultModel)
	}
}

func TestMergeSettingsPreservesSelfLearn(t *testing.T) {
	base := &Data{
		SelfLearn: SelfLearnSettings{
			Memory: SelfLearnMemory{Enabled: true, MaxKB: 15},
			Skills: SelfLearnSkills{DenyUpdate: true},
		},
	}
	overlay := &Data{
		SelfLearn: SelfLearnSettings{
			Skills:   SelfLearnSkills{DenyCreate: true},
			Strategy: "overlay strategy",
		},
	}
	got := mergeSettings(base, overlay)

	// Memory comes entirely from base since overlay didn't touch it.
	if !got.SelfLearn.Memory.Enabled || got.SelfLearn.Memory.MaxKB != 15 {
		t.Errorf("Memory: got %+v, want base passthrough", got.SelfLearn.Memory)
	}
	// Skills field-merges: Deny* OR across levels (overlay's DenyCreate + base's
	// DenyUpdate both survive); the shared Strategy coalesces from the overlay.
	if !got.SelfLearn.Skills.DenyCreate || !got.SelfLearn.Skills.DenyUpdate || got.SelfLearn.Strategy != "overlay strategy" {
		t.Errorf("Skills: got %+v, want merged overlay", got.SelfLearn.Skills)
	}

	// Symmetric: a base-only field survives an overlay that doesn't mention it.
	baseOnly := &Data{SelfLearn: SelfLearnSettings{Memory: SelfLearnMemory{Enabled: true}}}
	emptyOverlay := &Data{}
	got = mergeSettings(baseOnly, emptyOverlay)
	if !got.SelfLearn.Memory.Enabled {
		t.Errorf("base-only SelfLearn must survive empty overlay; got %+v", got.SelfLearn)
	}
}

// TestMergeSettingsCoalescesTokenLimit guards the tokenLimit merge: the
// overlay's non-zero value wins, and a base-only value survives an overlay
// that leaves it unset — otherwise the global context-window override would
// silently drop on every load/save merge.
func TestMergeSettingsCoalescesTokenLimit(t *testing.T) {
	// Overlay wins over base.
	got := mergeSettings(&Data{TokenLimit: 200000}, &Data{TokenLimit: 400000})
	if got.TokenLimit != 400000 {
		t.Errorf("overlay TokenLimit: got %d, want 400000", got.TokenLimit)
	}

	// Base survives an overlay that doesn't set it.
	got = mergeSettings(&Data{TokenLimit: 200000}, &Data{})
	if got.TokenLimit != 200000 {
		t.Errorf("base-only TokenLimit: got %d, want 200000", got.TokenLimit)
	}
}

// TestMergeSettingsDedupesSkillDirs guards the additive merge of the
// settings.json "skillDirs" list: user-level and project-level entries union
// (deduplicated, order-preserved), so a project can extend but not remove a
// user-level skill dir. Dropping this merge used to leave the field empty on
// every load, making extra skill dirs unreachable.
func TestMergeSettingsDedupesSkillDirs(t *testing.T) {
	got := mergeSettings(
		&Data{SkillDirs: []string{"/user/a", "/user/b"}},
		&Data{SkillDirs: []string{"/user/b", "/proj/c"}},
	)
	want := []string{"/user/a", "/user/b", "/proj/c"}
	if len(got.SkillDirs) != len(want) {
		t.Fatalf("SkillDirs: got %v, want %v", got.SkillDirs, want)
	}
	for i, w := range want {
		if got.SkillDirs[i] != w {
			t.Errorf("SkillDirs[%d]: got %q, want %q", i, got.SkillDirs[i], w)
		}
	}

	// base-only must survive an empty overlay
	got = mergeSettings(&Data{SkillDirs: []string{"/x"}}, &Data{})
	if len(got.SkillDirs) != 1 || got.SkillDirs[0] != "/x" {
		t.Errorf("base-only SkillDirs: got %v, want [/x]", got.SkillDirs)
	}
}
