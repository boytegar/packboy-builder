package lsp

import "os/exec"

// DefaultCatalog is a minimal built-in LSP server catalog keyed by server
// name. Entries are only used when no plugin contributes a server for the
// same extension, and only when the server binary is found on PATH.
//
// This mirrors Crush's approach (bundled defaults, PATH-gated auto-start)
// without shipping Crush's full 63 KB lsps.json. Add entries as the project
// validates them.
var DefaultCatalog = map[string]ServerConfig{
	"gopls": {
		Name:    "gopls",
		Command: "gopls",
		Args:    []string{"-mode=stdio"},
		ExtensionToLanguage: map[string]string{
			"go":  "go",
			"mod": "go",
			"sum": "go",
		},
	},
	"typescript-language-server": {
		Name:    "typescript-language-server",
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
		ExtensionToLanguage: map[string]string{
			"ts":  "typescript",
			"tsx": "typescriptreact",
			"js":  "javascript",
			"jsx": "javascriptreact",
		},
	},
	"pyright": {
		Name:    "pyright",
		Command: "pyright-langserver",
		Args:    []string{"--stdio"},
		ExtensionToLanguage: map[string]string{
			"py": "python",
		},
	},
	"rust-analyzer": {
		Name:    "rust-analyzer",
		Command: "rust-analyzer",
		Args:    nil,
		ExtensionToLanguage: map[string]string{
			"rs": "rust",
		},
	},
	"clangd": {
		Name:    "clangd",
		Command: "clangd",
		Args:    nil,
		ExtensionToLanguage: map[string]string{
			"c":   "c",
			"h":   "c",
			"cpp": "cpp",
			"cc":  "cpp",
			"hpp": "cpp",
		},
	},
	"lua-language-server": {
		Name:    "lua-language-server",
		Command: "lua-language-server",
		Args:    nil,
		ExtensionToLanguage: map[string]string{
			"lua": "lua",
		},
	},
	"zls": {
		Name:    "zls",
		Command: "zls",
		Args:    nil,
		ExtensionToLanguage: map[string]string{
			"zig": "zig",
		},
	},
}

// MergeWithDefaults returns a server map where plugin-contributed servers
// take priority, and default catalog entries are included only when their
// binary is available on PATH. Extensions already covered by a plugin
// server suppress the default for that extension set.
func MergeWithDefaults(pluginServers map[string]ServerConfig) map[string]ServerConfig {
	result := make(map[string]ServerConfig, len(pluginServers)+len(DefaultCatalog))

	// Plugin servers always win.
	coveredExt := make(map[string]bool)
	for name, cfg := range pluginServers {
		result[name] = cfg
		for ext := range cfg.ExtensionToLanguage {
			coveredExt[ext] = true
		}
	}

	// Add default entries whose binary exists and whose extensions are not
	// already covered by a plugin server.
	for name, cfg := range DefaultCatalog {
		if _, err := exec.LookPath(cfg.Command); err != nil {
			continue
		}
		// Skip if every extension is already covered by plugin servers.
		allCovered := true
		for ext := range cfg.ExtensionToLanguage {
			if !coveredExt[ext] {
				allCovered = false
				break
			}
		}
		if allCovered && len(cfg.ExtensionToLanguage) > 0 {
			continue
		}
		if _, exists := result[name]; !exists {
			result[name] = cfg
		}
	}

	return result
}
