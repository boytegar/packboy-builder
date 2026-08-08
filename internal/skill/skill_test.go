package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillStateNextState(t *testing.T) {
	tests := []struct {
		current  SkillState
		expected SkillState
	}{
		{StateDisable, StateEnable},
		{StateEnable, StateActive},
		{StateActive, StateDisable},
	}

	for _, tc := range tests {
		result := tc.current.NextState()
		if result != tc.expected {
			t.Errorf("NextState(%s) = %s, want %s", tc.current, result, tc.expected)
		}
	}
}

func TestSkillFullName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		expected  string
	}{
		{"commit", "git", "git:commit"},
		{"my-issues", "jira", "jira:my-issues"},
		{"test-skill", "", "test-skill"},
	}

	for _, tc := range tests {
		skill := &Skill{Name: tc.name, Namespace: tc.namespace}
		result := skill.FullName()
		if result != tc.expected {
			t.Errorf("FullName(%s, %s) = %s, want %s", tc.name, tc.namespace, result, tc.expected)
		}
	}
}

func TestLoadSkillFile(t *testing.T) {
	// Create a temporary skill file
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill
allowed-tools: [Read, Grep]
argument-hint: "[message]"
---

# Test Skill Instructions

This is the skill content.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := newLoader(tmpDir)
	skill, err := loader.loadSkillFile(skillPath, ScopeUser, "")
	if err != nil {
		t.Fatalf("loadSkillFile failed: %v", err)
	}

	if skill.Name != "test-skill" {
		t.Errorf("Name = %s, want test-skill", skill.Name)
	}
	if skill.Description != "A test skill" {
		t.Errorf("Description = %s, want 'A test skill'", skill.Description)
	}
	if skill.ArgumentHint != "[message]" {
		t.Errorf("ArgumentHint = %s, want '[message]'", skill.ArgumentHint)
	}
	if len(skill.AllowedTools) != 2 {
		t.Errorf("AllowedTools length = %d, want 2", len(skill.AllowedTools))
	}
	if skill.Scope != ScopeUser {
		t.Errorf("Scope = %d, want ScopeUser", skill.Scope)
	}
}

func TestLoadAllSkills(t *testing.T) {
	// Create temporary directories for skills
	tmpDir := t.TempDir()

	// Create a skill in the test directory
	skillDir := filepath.Join(tmpDir, ".pcb", "skills", "example-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: example-skill
description: An example skill
---

Example instructions.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create loader with the temp directory as project root
	loader := &loader{
		cwd: tmpDir,
	}

	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	skill, ok := skills["example-skill"]
	if !ok {
		t.Fatal("example-skill not found in loaded skills")
	}

	if skill.Description != "An example skill" {
		t.Errorf("Description = %s, want 'An example skill'", skill.Description)
	}
}

func TestLoadSkillWithNamespace(t *testing.T) {
	// Create temporary directories for skills
	tmpDir := t.TempDir()

	// Create a namespaced skill
	skillDir := filepath.Join(tmpDir, ".pcb", "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: commit
namespace: git
description: Create git commits
argument-hint: "[message]"
---

Commit instructions.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create loader with the temp directory as project root
	loader := &loader{
		cwd: tmpDir,
	}

	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	// Skill should be keyed by FullName (git:commit)
	skill, ok := skills["git:commit"]
	if !ok {
		t.Fatal("git:commit not found in loaded skills")
	}

	if skill.Name != "commit" {
		t.Errorf("Name = %s, want 'commit'", skill.Name)
	}
	if skill.Namespace != "git" {
		t.Errorf("Namespace = %s, want 'git'", skill.Namespace)
	}
	if skill.FullName() != "git:commit" {
		t.Errorf("FullName = %s, want 'git:commit'", skill.FullName())
	}
}

func TestSkillRegistry(t *testing.T) {
	// Create temporary directories for skills
	tmpDir := t.TempDir()

	// Create a skill in the test directory
	skillDir := filepath.Join(tmpDir, ".pcb", "skills", "registry-test")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: registry-test
description: Registry test skill
---

Test instructions.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override the loader to use our temp directory
	loader := &loader{
		cwd: tmpDir,
	}

	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	// Create mock stores in temp dir
	userStorePath := filepath.Join(tmpDir, "user-skills.json")
	projectStorePath := filepath.Join(tmpDir, "project-skills.json")
	userStore := &Store{
		path:   userStorePath,
		states: make(map[string]SkillState),
	}
	projectStore := &Store{
		path:   projectStorePath,
		states: make(map[string]SkillState),
	}

	registry := &Registry{
		skills:       skills,
		userStore:    userStore,
		projectStore: projectStore,
		cwd:          tmpDir,
	}

	// Test Get
	skill, ok := registry.Get("registry-test")
	if !ok {
		t.Fatal("registry-test not found")
	}
	if skill.State != StateEnable {
		t.Errorf("Default state = %s, want StateEnable", skill.State)
	}

	// Test SetState (to user level)
	err = registry.SetState("registry-test", StateActive, true)
	if err != nil {
		t.Fatalf("SetState failed: %v", err)
	}
	if skill.State != StateActive {
		t.Errorf("State after SetState = %s, want StateActive", skill.State)
	}

	// Test GetActive
	activeSkills := registry.GetActive()
	if len(activeSkills) != 1 {
		t.Errorf("GetActive returned %d skills, want 1", len(activeSkills))
	}

	// Test GetSkillsSection
	prompt := registry.GetSkillsSection()
	if prompt == "" {
		t.Error("GetSkillsSection returned empty string for active skill")
	}
	if !contains(prompt, "registry-test") {
		t.Error("Prompt should contain skill name")
	}
}

func TestMatchForPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, ".pcb", "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: commit
description: create a git commit
---

Commit instructions.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second skill that should not match generic prompts.
	skillDir2 := filepath.Join(tmpDir, ".pcb", "skills", "reviewer")
	if err := os.MkdirAll(skillDir2, 0o755); err != nil {
		t.Fatal(err)
	}
	skillPath2 := filepath.Join(skillDir2, "SKILL.md")
	skillContent2 := `---
name: reviewer
description: review code changes
---

Review instructions.
`
	if err := os.WriteFile(skillPath2, []byte(skillContent2), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := &loader{cwd: tmpDir}
	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}
	userStore := &Store{path: filepath.Join(tmpDir, "u.json"), states: make(map[string]SkillState)}
	projectStore := &Store{path: filepath.Join(tmpDir, "p.json"), states: make(map[string]SkillState)}
	registry := &Registry{skills: skills, userStore: userStore, projectStore: projectStore, cwd: tmpDir}

	// Set both to active so MatchForPrompt considers them.
	if err := registry.SetState("commit", StateActive, true); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetState("reviewer", StateActive, true); err != nil {
		t.Fatal(err)
	}

	// Match by skill name "commit" appearing in the prompt — but use a
	// prompt that contains no words from the reviewer skill's description
	// ("review", "code", "changes") to keep it unambiguous.
	matches := registry.MatchForPrompt("please commit my latest work")
	if len(matches) != 1 {
		t.Fatalf("name match: got %d matches, want 1", len(matches))
	}

	// Match by description keyword "review" for the reviewer skill.
	matches = registry.MatchForPrompt("can you review this pull request?")
	if len(matches) != 0 {
		t.Fatalf("description-only match: got %d matches, want 0", len(matches))
	}

	matches = registry.MatchForPrompt("please ask the reviewer to review this")
	if len(matches) != 1 {
		t.Fatalf("exact name match: got %d matches, want 1", len(matches))
	}

	// No match for a prompt mentioning neither skill's keywords.
	matches = registry.MatchForPrompt("what is the weather today")
	if len(matches) != 0 {
		t.Fatalf("no-match: got %d matches, want 0", len(matches))
	}

	// Empty prompt returns nil.
	if matches := registry.MatchForPrompt(""); matches != nil {
		t.Fatalf("empty prompt: got %v, want nil", matches)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoadPluginSkills(t *testing.T) {
	tmpDir := t.TempDir()

	// A plugin contributes one skill directory containing a SKILL.md.
	skillDir := filepath.Join(tmpDir, "git-plugin", "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: commit
description: Create git commits
argument-hint: "[message]"
---

Git commit instructions.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// The app injects enabled-plugin skill directories; here a project-scope
	// plugin named "git" contributes the commit skill.
	loader := &loader{
		cwd: tmpDir,
		pluginSkillPaths: func() []PluginSkillPath {
			return []PluginSkillPath{{Path: skillDir, Namespace: "git", IsProject: true}}
		},
	}

	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	// Skill should inherit namespace from plugin name (git)
	skill, ok := skills["git:commit"]
	if !ok {
		t.Fatal("git:commit not found in loaded skills - namespace inheritance may not be working")
	}

	if skill.Name != "commit" {
		t.Errorf("Name = %s, want 'commit'", skill.Name)
	}
	if skill.Namespace != "git" {
		t.Errorf("Namespace = %s, want 'git' (inherited from plugin name)", skill.Namespace)
	}
	if skill.FullName() != "git:commit" {
		t.Errorf("FullName = %s, want 'git:commit'", skill.FullName())
	}
	if skill.Scope != ScopeProjectPlugin {
		t.Errorf("Scope = %s, want ScopeProjectPlugin", skill.Scope.String())
	}
}

func TestPluginSkillExplicitNamespaceOverride(t *testing.T) {
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "my-plugin", "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: review
namespace: code
description: Code review skill
---

Review instructions.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Plugin "my-plugin" contributes the skill, but the skill's frontmatter
	// declares an explicit namespace "code" that must win.
	loader := &loader{
		cwd: tmpDir,
		pluginSkillPaths: func() []PluginSkillPath {
			return []PluginSkillPath{{Path: skillDir, Namespace: "my-plugin", IsProject: true}}
		},
	}

	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	// Skill should use explicit namespace (code) not plugin name (my-plugin)
	skill, ok := skills["code:review"]
	if !ok {
		t.Fatal("code:review not found in loaded skills - explicit namespace should override plugin name")
	}

	if skill.Namespace != "code" {
		t.Errorf("Namespace = %s, want 'code' (explicit frontmatter)", skill.Namespace)
	}
}

// writeSkillHelper creates a SKILL.md with the given name under dir.
func writeSkillHelper(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + name + " skill\n---\n\nInstructions.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtraSkillDirsLoaded(t *testing.T) {
	tmpDir := t.TempDir()

	// Extra skill dir from settings.json "skillDirs"
	extra := filepath.Join(tmpDir, "team-skills", "planning")
	writeSkillHelper(t, extra, "planning")

	loader := &loader{
		cwd:             tmpDir,
		extraSkillPaths: []string{filepath.Join(tmpDir, "team-skills")},
	}

	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}
	sk, ok := skills["planning"]
	if !ok {
		t.Fatal("planning skill from extraSkillPaths not loaded")
	}
	if sk.Scope != ScopeCustom {
		t.Errorf("Scope = %s, want ScopeCustom", sk.Scope.String())
	}
}

func TestExtraSkillDirOverridesUserScope(t *testing.T) {
	// An extra (ScopeCustom) skill should override a ~/.pcb/skills (ScopeUser)
	// skill of the same name, since ScopeCustom > ScopeUser.
	tmpDir := t.TempDir()

	// User-level skill via confdir under home — build the path the loader uses.
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		t.Skip("no home dir")
	}
	userSkillDir := filepath.Join(homeDir, ".pcb", "skills", "shared")
	// Best-effort: only write if writable; skip otherwise.
	if err := os.MkdirAll(userSkillDir, 0o755); err != nil {
		t.Skipf("cannot create user skill dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(homeDir, ".pcb", "skills", "shared")) })
	writeSkillHelper(t, userSkillDir, "shared")

	// Extra dir with a same-named skill that should win.
	extraRoot := filepath.Join(tmpDir, "team-skills")
	writeSkillHelper(t, filepath.Join(extraRoot, "shared"), "shared")

	loader := &loader{
		cwd:             tmpDir,
		extraSkillPaths: []string{extraRoot},
	}
	skills, _ := loader.loadAll()
	sk, ok := skills["shared"]
	if !ok {
		t.Fatal("shared skill not found")
	}
	if sk.Scope != ScopeCustom {
		t.Errorf("Scope = %s, want ScopeCustom (extra should override user)", sk.Scope.String())
	}
}

func TestExtraSkillDirYieldsToProjectScope(t *testing.T) {
	// A .pcb/skills (ScopeProject) skill should override an extra (ScopeCustom)
	// skill of the same name, since ScopeProject > ScopeCustom.
	tmpDir := t.TempDir()

	extraRoot := filepath.Join(tmpDir, "team-skills")
	writeSkillHelper(t, filepath.Join(extraRoot, "alpha"), "alpha")

	projSkillDir := filepath.Join(tmpDir, ".pcb", "skills", "alpha")
	writeSkillHelper(t, projSkillDir, "alpha")

	loader := &loader{
		cwd:             tmpDir,
		extraSkillPaths: []string{extraRoot},
	}
	skills, _ := loader.loadAll()
	sk, ok := skills["alpha"]
	if !ok {
		t.Fatal("alpha skill not found")
	}
	if sk.Scope != ScopeProject {
		t.Errorf("Scope = %s, want ScopeProject (project should override extra)", sk.Scope.String())
	}
}

func TestResolveSkillDirs(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	cwd := "/work/proj"

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"blanks dropped", []string{"", "  ", "/abs"}, []string{"/abs"}},
		{"absolute passthrough", []string{"/mnt/shared/skills"}, []string{"/mnt/shared/skills"}},
		{"relative joined to cwd", []string{"./skills"}, []string{"/work/proj/skills"}},
		{"tilde expands to home", []string{"~/team-skills"}, []string{filepath.Join(homeDir, "team-skills")}},
		{"bare tilde expands to home", []string{"~"}, []string{homeDir}},
		{"dedup", []string{"/a", "/a", "/b"}, []string{"/a", "/b"}},
		{"whitespace trimmed", []string{"  /x  "}, []string{"/x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSkillDirs(tc.in, cwd)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSkillsSectionIsEscapedCatalog(t *testing.T) {
	registry := NewRegistryForTest(map[string]*Skill{
		"alpha": {
			Name:         "alpha",
			Description:  `Use <fast> & "safe"`,
			ArgumentHint: `<file>`,
			FilePath:     `/tmp/a&b/SKILL.md`,
			State:        StateActive,
		},
		"hidden": {Name: "hidden", Description: "no", State: StateEnable},
	}, &Store{states: map[string]SkillState{}}, &Store{states: map[string]SkillState{}})

	section := registry.GetSkillsSection()
	for _, want := range []string{
		"<available_skills>",
		"<name>alpha</name>",
		"Use &lt;fast&gt; &amp; &quot;safe&quot;",
		"<argument_hint>&lt;file&gt;</argument_hint>",
		"/tmp/a&amp;b/SKILL.md",
		"<skills_usage>",
	} {
		if !contains(section, want) {
			t.Errorf("catalog missing %q:\n%s", want, section)
		}
	}
	if contains(section, "hidden") {
		t.Fatalf("enable-only skill leaked into catalog: %s", section)
	}
}
