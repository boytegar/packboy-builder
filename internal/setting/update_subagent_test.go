package setting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateSubagentModelAtSetAndClear(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	file := userSettingsFile(home)

	read := func() map[string]string {
		t.Helper()
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read user settings: %v", err)
		}
		var d Data
		if err := json.Unmarshal(data, &d); err != nil {
			t.Fatalf("unmarshal user settings: %v", err)
		}
		return d.SubagentModels
	}

	// Set an override.
	if err := UpdateSubagentModelAt("coder", "deepseek/deepseek-v4", true); err != nil {
		t.Fatalf("set: %v", err)
	}
	if m := read(); m["coder"] != "deepseek/deepseek-v4" {
		t.Fatalf("after set, want coder=deepseek/deepseek-v4, got %q", m["coder"])
	}

	// Setting a second agent keeps the first.
	if err := UpdateSubagentModelAt("reviewer", "anthropic/claude-sonnet-4-6", true); err != nil {
		t.Fatalf("set second: %v", err)
	}
	if m := read(); len(m) != 2 {
		t.Fatalf("after second set, want 2 overrides, got %d", len(m))
	}

	// "inherit" clears the override (deletes the key).
	if err := UpdateSubagentModelAt("coder", "inherit", true); err != nil {
		t.Fatalf("clear: %v", err)
	}
	m := read()
	if _, ok := m["coder"]; ok {
		t.Fatalf("after inherit, coder should be absent, got %q", m["coder"])
	}
	if m["reviewer"] != "anthropic/claude-sonnet-4-6" {
		t.Fatalf("clear should preserve reviewer, got %q", m["reviewer"])
	}

	// "" also clears.
	if err := UpdateSubagentModelAt("reviewer", "", true); err != nil {
		t.Fatalf("clear empty: %v", err)
	}
	if m := read(); len(m) != 0 {
		t.Fatalf("after empty clear, want 0 overrides, got %d (%v)", len(m), m)
	}
}

func TestUpdateSubagentModelAtPreservesOtherSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	file := userSettingsFile(home)

	// Pre-existing settings on disk.
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`{"theme":"dark","model":"gpt-5"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateSubagentModelAt("coder", "haiku", true); err != nil {
		t.Fatalf("set: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var d Data
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Theme != "dark" {
		t.Fatalf("theme lost: %q", d.Theme)
	}
	if d.Model != "gpt-5" {
		t.Fatalf("model lost: %q", d.Model)
	}
	if d.SubagentModels["coder"] != "haiku" {
		t.Fatalf("override missing: %q", d.SubagentModels["coder"])
	}
}
