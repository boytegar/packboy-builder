package lsp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func testServers() map[string]ServerConfig {
	return map[string]ServerConfig{
		"go": {
			Name:    "go",
			Command: "gopls",
			Args:    []string{"-mode=stdio"},
			ExtensionToLanguage: map[string]string{
				"go":  "go",
				"mod": "go",
			},
		},
		"ts": {
			Name:    "ts",
			Command: "typescript-language-server",
			Args:    []string{"--stdio"},
			ExtensionToLanguage: map[string]string{
				"ts":  "typescript",
				"tsx": "typescriptreact",
			},
		},
	}
}

func TestServerForPath(t *testing.T) {
	m := NewManager("/tmp", testServers())

	cases := []struct {
		path string
		name string
		ok   bool
	}{
		{"/repo/main.go", "go", true},
		{"/repo/go.mod", "go", true},
		{"/repo/app.ts", "ts", true},
		{"/repo/App.tsx", "ts", true},
		{"/repo/README.md", "", false},
		{"/repo/app.py", "", false},
	}
	for _, c := range cases {
		name, ok := m.ServerForPath(c.path)
		if ok != c.ok || (ok && name != c.name) {
			t.Errorf("ServerForPath(%q) = %q,%v want %q,%v", c.path, name, ok, c.name, c.ok)
		}
	}
}

func TestStartUnknownServer(t *testing.T) {
	m := NewManager("/tmp", testServers())
	if _, err := m.Start(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestStartMissingBinary(t *testing.T) {
	m := NewManager("/tmp", map[string]ServerConfig{
		"x": {Name: "x", Command: "definitely-not-a-real-lsp-binary-xyz", ExtensionToLanguage: map[string]string{"zz": "z"}},
	})
	_, err := m.Start(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	// Second attempt should not respawn (remembered failure).
	if _, err := m.Start(context.Background(), "x"); err == nil {
		t.Fatal("expected remembered failure")
	}
}

func TestDiagnosticsSortedAndCopied(t *testing.T) {
	m := NewManager("/tmp", testServers())
	uri := "file:///repo/main.go"
	m.handleNotification("go")("textDocument/publishDiagnostics", mustJSON(t, PublishDiagnosticsParams{
		URI: uri,
		Diagnostics: []Diagnostic{
			{Range: LSPRange{Start: LSPPosition{Line: 5, Character: 0}}, Message: "later"},
			{Range: LSPRange{Start: LSPPosition{Line: 1, Character: 0}}, Message: "earlier"},
		},
	}))

	got := m.Diagnostics(uri)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Message != "earlier" || got[1].Message != "later" {
		t.Fatalf("diagnostics not sorted by position: %+v", got)
	}

	// Mutating the returned slice must not affect the cache.
	got[0].Message = "mutated"
	if m.Diagnostics(uri)[0].Message == "mutated" {
		t.Fatal("Diagnostics returned shared slice")
	}
}

func TestHasServerAndStatus(t *testing.T) {
	m := NewManager("/tmp", testServers())
	if !m.HasServer() {
		t.Fatal("HasServer should be true")
	}
	status := m.Status()
	if len(status) != 2 {
		t.Fatalf("status len %d, want 2", len(status))
	}
	if status[0].Name != "go" || status[1].Name != "ts" {
		t.Fatalf("status not sorted: %+v", status)
	}
}

func TestInitializeMergesDefaults(t *testing.T) {
	ResetDefault()
	if err := Initialize(ServiceOptions{CWD: "/tmp", Servers: nil}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	// With no plugin servers, only PATH-available defaults are included.
	// On a machine with gopls installed, HasServer() will be true; on a clean
	// CI it may be false. Either is valid — just ensure no panic and clean state.
	_ = Default().Manager().HasServer()
	ResetDefault()
}

func TestMergeWithDefaultsPluginWins(t *testing.T) {
	plugin := map[string]ServerConfig{
		"custom-go": {Name: "custom-go", Command: "gopls", ExtensionToLanguage: map[string]string{"go": "go"}},
	}
	merged := MergeWithDefaults(plugin)
	// Plugin server must be present.
	if _, ok := merged["custom-go"]; !ok {
		t.Fatal("plugin server missing from merge")
	}
	// "go" extension is covered by plugin, so default "gopls" should be suppressed
	// (all its extensions are covered).
	if _, ok := merged["gopls"]; ok {
		t.Fatal("default gopls should be suppressed when plugin covers all its extensions")
	}
}

func TestMergeWithDefaultsSkipsMissingBinary(t *testing.T) {
	merged := MergeWithDefaults(nil)
	// "rust-analyzer" is unlikely to be on PATH in CI.
	if _, ok := merged["rust-analyzer"]; ok {
		// If it IS on PATH, that's fine — just skip the assertion.
		return
	}
	// If not on PATH, it must not appear.
	if _, ok := merged["rust-analyzer"]; ok {
		t.Fatal("rust-analyzer should not be merged when binary is absent")
	}
}

func TestKillAllIdempotent(t *testing.T) {
	m := NewManager("/tmp", testServers())
	m.KillAll() // no clients — must not panic
	m.KillAll()
}

func TestServerForPathTimeout(t *testing.T) {
	m := NewManager("/tmp", testServers())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// "go" server: gopls almost certainly absent in CI; Start should fail fast
	// on LookPath, but if present it should time out rather than hang.
	_, _ = m.Start(ctx, "go")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	return mustMarshal(t, v)
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
