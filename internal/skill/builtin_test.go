package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinGraphSkillLoads(t *testing.T) {
	root, err := materializeBuiltinSkills()
	if err != nil {
		t.Fatalf("materializeBuiltinSkills: %v", err)
	}
	if root == "" {
		t.Fatal("expected non-empty materialize root")
	}

	loader := newLoader(t.TempDir())
	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll: %v", err)
	}

	sk, ok := skills["graph"]
	if !ok {
		t.Fatal("builtin graph skill missing from loadAll")
	}
	if sk.Scope != ScopeBuiltin {
		t.Fatalf("Scope = %s, want builtin", sk.Scope)
	}
	if sk.Name != "graph" {
		t.Fatalf("Name = %q, want graph", sk.Name)
	}
	if sk.ArgumentHint == "" {
		t.Fatal("ArgumentHint should be set for /graph usage")
	}
	if len(sk.References) == 0 {
		t.Fatal("graph skill should ship reference files")
	}

	body := sk.GetInstructions()
	for _, want := range []string{"9-Stage Pipeline", "Teaching Mode", "Ontology modeling"} {
		if !strings.Contains(body, want) {
			t.Fatalf("instructions missing %q (len=%d)", want, len(body))
		}
	}
}

func TestBuiltinSpecSkillLoads(t *testing.T) {
	root, err := materializeBuiltinSkills()
	if err != nil {
		t.Fatalf("materializeBuiltinSkills: %v", err)
	}
	if root == "" {
		t.Fatal("expected non-empty materialize root")
	}

	loader := newLoader(t.TempDir())
	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll: %v", err)
	}

	sk, ok := skills["spec"]
	if !ok {
		t.Fatal("builtin spec skill missing from loadAll")
	}
	if sk.Scope != ScopeBuiltin {
		t.Fatalf("Scope = %s, want builtin", sk.Scope)
	}
	if sk.Name != "spec" {
		t.Fatalf("Name = %q, want spec", sk.Name)
	}
	if sk.ArgumentHint == "" {
		t.Fatal("ArgumentHint should be set for /spec usage")
	}
	if len(sk.References) == 0 {
		t.Fatal("spec skill should ship reference files")
	}

	body := sk.GetInstructions()
	for _, want := range []string{"Spec Generation", "Phase 1", "Phase 2", "context/"} {
		if !strings.Contains(body, want) {
			t.Fatalf("instructions missing %q (len=%d)", want, len(body))
		}
	}
}

func TestProjectSkillShadowsBuiltin(t *testing.T) {
	tmp := t.TempDir()
	skillDir := tmp + "/.pcb/skills/graph"
	if err := writeTestSkill(skillDir, "graph", "project override body"); err != nil {
		t.Fatal(err)
	}

	loader := newLoader(tmp)
	skills, err := loader.loadAll()
	if err != nil {
		t.Fatalf("loadAll: %v", err)
	}
	sk, ok := skills["graph"]
	if !ok {
		t.Fatal("graph skill missing")
	}
	if sk.Scope != ScopeProject {
		t.Fatalf("Scope = %s, want project (shadow builtin)", sk.Scope)
	}
	if got := sk.GetInstructions(); !strings.Contains(got, "project override body") {
		t.Fatalf("expected project body, got %q", got)
	}
}

func writeTestSkill(dir, name, body string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := "---\nname: " + name + "\ndescription: test\n---\n\n" + body + "\n"
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}
