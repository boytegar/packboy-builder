package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/boytegar/packboy-builder/internal/atomicfile"
	"github.com/boytegar/packboy-builder/internal/confdir"
)

// Scope mirrors mcp.Scope: where the config file lives.
type Scope string

const (
	ScopeUser    Scope = "user"    // ~/.pcb/lsp.json (global)
	ScopeProject Scope = "project" // ./.pcb/lsp.json (team shared)
	ScopeLocal   Scope = "local"   // ./.pcb/lsp.local.json (personal, git-ignored)
)

// FileServerConfig is the JSON shape for a single LSP server in lsp.json.
// It is richer than plugin's LSPServerConfig: supports env and disabled.
type FileServerConfig struct {
	Command             string            `json:"command"`
	Args                []string          `json:"args,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	Disabled            bool              `json:"disabled,omitempty"`
	ExtensionToLanguage map[string]string `json:"extensionToLanguage,omitempty"`
	// InitOptions is JSON passthrough for the server's initializationOptions.
	InitOptions json.RawMessage `json:"initOptions,omitempty"`
}

// LSPFileConfig is the top-level JSON shape: {"lspServers": {...}}.
type LSPFileConfig struct {
	LSPServers map[string]FileServerConfig `json:"lspServers"`
}

// ConfigLoader loads LSP server configs from user/project/local JSON files,
// mirroring the MCP config loader pattern.
type ConfigLoader struct {
	userDir    string // ~/.pcb
	projectDir string // ./.pcb or cwd
}

func NewConfigLoader(cwd string) *ConfigLoader {
	homeDir, _ := os.UserHomeDir()
	return &ConfigLoader{
		userDir:    confdir.Dir(homeDir),
		projectDir: confdir.Dir(cwd),
	}
}

func NewConfigLoaderForTest(baseDir string) *ConfigLoader {
	return &ConfigLoader{
		userDir:    confdir.Dir(filepath.Join(baseDir, "user")),
		projectDir: confdir.Dir(filepath.Join(baseDir, "project")),
	}
}

// LoadAll loads and merges LSP configs from all sources in priority order:
//  1. ~/.pcb/lsp.json (user scope)
//  2. ./.pcb/lsp.json (project scope)
//  3. ./.pcb/lsp.local.json (local scope)
//
// Later sources override earlier ones for the same server name.
// Disabled servers are filtered out.
func (l *ConfigLoader) LoadAll() (map[string]ServerConfig, error) {
	servers := make(map[string]ServerConfig)

	sources := []struct {
		path  string
		scope Scope
	}{
		{filepath.Join(l.userDir, "lsp.json"), ScopeUser},
		{filepath.Join(l.projectDir, "lsp.json"), ScopeProject},
		{filepath.Join(l.projectDir, "lsp.local.json"), ScopeLocal},
	}

	for _, src := range sources {
		configs, err := l.loadFile(src.path)
		if err != nil {
			continue // missing file is not an error
		}
		for name, cfg := range configs {
			if cfg.Disabled {
				delete(servers, name)
				continue
			}
			servers[name] = ServerConfig{
				Name:                name,
				Command:             cfg.Command,
				Args:                cfg.Args,
				ExtensionToLanguage: cfg.ExtensionToLanguage,
			}
		}
	}

	return servers, nil
}

func (l *ConfigLoader) loadFile(path string) (map[string]FileServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config LSPFileConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("lsp: parse %s: %w", path, err)
	}
	return config.LSPServers, nil
}

// GetFilePath returns the config file path for a given scope.
func (l *ConfigLoader) GetFilePath(scope Scope) string {
	switch scope {
	case ScopeUser:
		return filepath.Join(l.userDir, "lsp.json")
	case ScopeProject:
		return filepath.Join(l.projectDir, "lsp.json")
	case ScopeLocal:
		return filepath.Join(l.projectDir, "lsp.local.json")
	default:
		return filepath.Join(l.projectDir, "lsp.local.json")
	}
}

// SaveServer writes a server config to the file for the given scope,
// preserving other servers already in that file.
func (l *ConfigLoader) SaveServer(name string, config FileServerConfig, scope Scope) error {
	filePath := l.GetFilePath(scope)

	existing := make(map[string]FileServerConfig)
	if data, err := os.ReadFile(filePath); err == nil {
		var fileCfg LSPFileConfig
		if err := json.Unmarshal(data, &fileCfg); err == nil && fileCfg.LSPServers != nil {
			existing = fileCfg.LSPServers
		}
	}
	existing[name] = config

	fileCfg := LSPFileConfig{LSPServers: existing}
	data, err := json.MarshalIndent(fileCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("lsp: marshal config: %w", err)
	}
	return atomicfile.Write(filePath, data, 0644)
}

// RemoveServer removes a server config from the file for the given scope.
func (l *ConfigLoader) RemoveServer(name string, scope Scope) error {
	filePath := l.GetFilePath(scope)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil // already gone
	}
	var fileCfg LSPFileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("lsp: parse %s: %w", filePath, err)
	}
	if fileCfg.LSPServers == nil {
		return nil
	}
	delete(fileCfg.LSPServers, name)

	out, err := json.MarshalIndent(fileCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("lsp: marshal config: %w", err)
	}
	return atomicfile.Write(filePath, out, 0644)
}
