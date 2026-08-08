package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoaderLoadAll(t *testing.T) {
	base := t.TempDir()
	loader := NewConfigLoaderForTest(base)

	// User scope: ~/.pcb/lsp.json
	userDir := filepath.Join(base, "user", ".pcb")
	_ = os.MkdirAll(userDir, 0755)
	writeJSON(t, filepath.Join(userDir, "lsp.json"), LSPFileConfig{
		LSPServers: map[string]FileServerConfig{
			"gopls": {Command: "gopls", Args: []string{"-mode=stdio"}, ExtensionToLanguage: map[string]string{"go": "go"}},
		},
	})

	// Project scope: ./.pcb/lsp.json (overrides user)
	projDir := filepath.Join(base, "project", ".pcb")
	_ = os.MkdirAll(projDir, 0755)
	writeJSON(t, filepath.Join(projDir, "lsp.json"), LSPFileConfig{
		LSPServers: map[string]FileServerConfig{
			"gopls":   {Command: "gopls-custom", ExtensionToLanguage: map[string]string{"go": "go"}}, // override
			"pyright": {Command: "pyright-langserver", ExtensionToLanguage: map[string]string{"py": "python"}},
		},
	})

	// Local scope: ./.pcb/lsp.local.json
	writeJSON(t, filepath.Join(projDir, "lsp.local.json"), LSPFileConfig{
		LSPServers: map[string]FileServerConfig{
			"rust-analyzer": {Command: "rust-analyzer", ExtensionToLanguage: map[string]string{"rs": "rust"}},
		},
	})

	servers, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// gopls should be overridden by project scope.
	if cfg, ok := servers["gopls"]; !ok {
		t.Fatal("gopls missing")
	} else if cfg.Command != "gopls-custom" {
		t.Fatalf("gopls command = %q, want gopls-custom", cfg.Command)
	}

	// pyright from project scope.
	if _, ok := servers["pyright"]; !ok {
		t.Fatal("pyright missing")
	}

	// rust-analyzer from local scope.
	if _, ok := servers["rust-analyzer"]; !ok {
		t.Fatal("rust-analyzer missing")
	}
}

func TestConfigLoaderDisabledFiltered(t *testing.T) {
	base := t.TempDir()
	loader := NewConfigLoaderForTest(base)

	userDir := filepath.Join(base, "user", ".pcb")
	_ = os.MkdirAll(userDir, 0755)
	writeJSON(t, filepath.Join(userDir, "lsp.json"), LSPFileConfig{
		LSPServers: map[string]FileServerConfig{
			"active":   {Command: "echo", ExtensionToLanguage: map[string]string{"x": "x"}},
			"inactive": {Command: "echo", Disabled: true, ExtensionToLanguage: map[string]string{"y": "y"}},
		},
	})

	servers, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if _, ok := servers["active"]; !ok {
		t.Fatal("active server should be present")
	}
	if _, ok := servers["inactive"]; ok {
		t.Fatal("disabled server should be filtered out")
	}
}

func TestConfigLoaderMissingFiles(t *testing.T) {
	base := t.TempDir()
	loader := NewConfigLoaderForTest(base)

	servers, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll with no files: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(servers))
	}
}

func TestConfigLoaderSaveAndRemove(t *testing.T) {
	base := t.TempDir()
	loader := NewConfigLoaderForTest(base)

	// Save a server.
	err := loader.SaveServer("gopls", FileServerConfig{
		Command:             "gopls",
		Args:                []string{"-mode=stdio"},
		ExtensionToLanguage: map[string]string{"go": "go"},
	}, ScopeProject)
	if err != nil {
		t.Fatalf("SaveServer: %v", err)
	}

	// Verify it loads.
	servers, _ := loader.LoadAll()
	if _, ok := servers["gopls"]; !ok {
		t.Fatal("saved server not found")
	}

	// Remove it.
	if err := loader.RemoveServer("gopls", ScopeProject); err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	servers, _ = loader.LoadAll()
	if _, ok := servers["gopls"]; ok {
		t.Fatal("removed server should be gone")
	}
}

func TestConfigLoaderGetFilePath(t *testing.T) {
	base := t.TempDir()
	loader := NewConfigLoaderForTest(base)

	cases := []struct {
		scope Scope
		suf   string
	}{
		{ScopeUser, "user/.pcb/lsp.json"},
		{ScopeProject, "project/.pcb/lsp.json"},
		{ScopeLocal, "project/.pcb/lsp.local.json"},
	}
	for _, c := range cases {
		got := loader.GetFilePath(c.scope)
		want := filepath.Join(base, c.suf)
		if got != want {
			t.Errorf("GetFilePath(%s) = %q, want %q", c.scope, got, want)
		}
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
