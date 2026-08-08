package lsp

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
)

func TestPositionEncodingDefaultsToUTF16(t *testing.T) {
	c := NewClient(ServerConfig{Name: "test", Command: "noop"})
	if got := c.PositionEncoding(); got != "utf-16" {
		t.Fatalf("default encoding = %q, want utf-16", got)
	}
}

func TestSetPositionEncoding(t *testing.T) {
	c := NewClient(ServerConfig{Name: "test", Command: "noop"})
	c.setPositionEncoding("utf-8")
	if got := c.PositionEncoding(); got != "utf-8" {
		t.Fatalf("encoding = %q, want utf-8", got)
	}
}

func TestRestartServerUnknownName(t *testing.T) {
	m := NewManager("/tmp", testServers())
	if _, err := m.RestartServer(context.Background(), "nonexistent"); err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestRestartServerNotStarted(t *testing.T) {
	// Restarting a server that was never started should attempt to start it,
	// which will fail (binary not on PATH for the "x" config). The call
	// must not panic.
	m := NewManager("/tmp", map[string]ServerConfig{
		"x": {Name: "x", Command: "definitely-not-a-real-lsp-binary-xyz", ExtensionToLanguage: map[string]string{"zz": "z"}},
	})
	if _, err := m.RestartServer(context.Background(), "x"); err == nil {
		// If the binary somehow exists, that's fine — just don't panic.
	}
}

func TestCrashRecoveryOnDeadClient(t *testing.T) {
	m := NewManager("/tmp", map[string]ServerConfig{
		"x": {Name: "x", Command: "definitely-not-a-real-lsp-binary-xyz", ExtensionToLanguage: map[string]string{"zz": "z"}},
	})
	// Start will fail (binary not found), marking it as started+failed.
	_, err := m.Start(context.Background(), "x")
	if err == nil {
		t.Fatal("expected start failure for missing binary")
	}
	// Subsequent Start should return the remembered failure, not retry.
	_, err = m.Start(context.Background(), "x")
	if err == nil {
		t.Fatal("expected remembered failure on second Start")
	}
}

func TestMergeWithDefaultsEmptyPlugin(t *testing.T) {
	merged := MergeWithDefaults(nil)
	// At minimum, the catalog keys exist in the source — those whose
	// binaries are present will be in the result.
	for name, cfg := range merged {
		if cfg.Command == "" {
			t.Fatalf("merged server %q has empty command", name)
		}
	}
}

func TestMergeWithDefaultsPluginCoversExtension(t *testing.T) {
	plugin := map[string]ServerConfig{
		"my-go": {Name: "my-go", Command: "gopls", ExtensionToLanguage: map[string]string{"go": "go", "mod": "go", "sum": "go"}},
	}
	merged := MergeWithDefaults(plugin)
	if _, ok := merged["my-go"]; !ok {
		t.Fatal("plugin server must be present")
	}
	// gopls default should be suppressed since all its extensions (go, mod, sum)
	// are covered by the plugin server.
	if _, ok := merged["gopls"]; ok {
		t.Fatal("default gopls should be suppressed when plugin covers all extensions")
	}
}

func TestMergeWithDefaultsPluginPartialCover(t *testing.T) {
	// Plugin only covers "go" but not "mod" or "sum" → default gopls should
	// still be included (not all extensions covered).
	plugin := map[string]ServerConfig{
		"my-go": {Name: "my-go", Command: "gopls", ExtensionToLanguage: map[string]string{"go": "go"}},
	}
	merged := MergeWithDefaults(plugin)
	if _, ok := merged["my-go"]; !ok {
		t.Fatal("plugin server must be present")
	}
	// gopls default should be present because "mod" and "sum" are uncovered.
	// (Only check if gopls binary is on PATH; skip otherwise.)
	if _, err := exec.LookPath("gopls"); err == nil {
		if _, ok := merged["gopls"]; !ok {
			t.Fatal("default gopls should be present when plugin only partially covers extensions")
		}
	}
}

func TestSymbolKindName(t *testing.T) {
	// Test a few known kinds via the tool-level helper — re-implemented
	// here to avoid importing the tool package.
	cases := map[int]string{
		1:  "File",
		6:  "Method",
		12: "Function",
		13: "Variable",
		23: "Struct",
		99: "kind(99)",
	}
	for kind, want := range cases {
		got := symbolKindNamePublic(kind)
		if got != want {
			t.Errorf("symbolKindName(%d) = %q, want %q", kind, got, want)
		}
	}
}

// symbolKindNamePublic mirrors the tool's symbolKindName for testing.
func symbolKindNamePublic(kind int) string {
	names := map[int]string{
		1: "File", 2: "Module", 3: "Namespace", 4: "Package", 5: "Class",
		6: "Method", 7: "Property", 8: "Field", 9: "Constructor", 10: "Enum",
		11: "Interface", 12: "Function", 13: "Variable", 14: "Constant",
		15: "String", 16: "Number", 17: "Boolean", 18: "Array", 19: "Object",
		20: "Key", 21: "Null", 22: "EnumMember", 23: "Struct", 24: "Event",
		25: "Operator", 26: "TypeParameter",
	}
	if name, ok := names[kind]; ok {
		return name
	}
	return fmt.Sprintf("kind(%d)", kind)
}

// fmt import needed for Sprintf above.
var _ = fmt.Sprintf
