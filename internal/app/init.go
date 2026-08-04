package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/agent"
	"github.com/boytegar/packboy-builder/internal/app/input"
	"github.com/boytegar/packboy-builder/internal/app/kit/suggest"
	"github.com/boytegar/packboy-builder/internal/command"
	"github.com/boytegar/packboy-builder/internal/confdir"
	"github.com/boytegar/packboy-builder/internal/cron"
	"github.com/boytegar/packboy-builder/internal/hook"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/log"
	"github.com/boytegar/packboy-builder/internal/mcp"
	"github.com/boytegar/packboy-builder/internal/persona"
	"github.com/boytegar/packboy-builder/internal/plugin"
	"github.com/boytegar/packboy-builder/internal/session"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/skill"
	"github.com/boytegar/packboy-builder/internal/subagent"
	"github.com/boytegar/packboy-builder/internal/task"
	"github.com/boytegar/packboy-builder/internal/todo"
	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/fs"
	_ "github.com/boytegar/packboy-builder/internal/tool/registry"
)

var appCwd string

func initInfrastructure() error {
	appCwd, _ = os.Getwd()

	// Phase 1: foundation — no cross-service deps
	setting.Initialize(setting.Options{CWD: appCwd})
	llm.Initialize(llm.Options{})

	// Phase 2: extensions — plugin first, then dependents
	initExtensions(appCwd)

	// Phase 3: tool infrastructure
	tool.Initialize(tool.Options{})
	agent.Initialize(agent.Options{})
	task.Initialize(task.Options{})
	todo.Initialize(todo.Options{})
	cron.Initialize(cron.Options{
		StoragePath: filepath.Join(confdir.Dir(appCwd), "scheduled_tasks.json"),
	})
	if err := cron.Default().LoadDurable(); err != nil {
		return fmt.Errorf("failed to load scheduled tasks: %w", err)
	}
	fs.SetEnvProvider(plugin.PluginEnv)
	// Cross-goroutine fallback: when an agent goroutine spawns a hook
	// script or bash subprocess and ctx wasn't pre-loaded with the
	// active plugin root, plugin.PluginEnv falls back to this provider.
	// It reads the foreground task's per-turn plugin scope.
	plugin.SetRootProvider(func() string {
		return agent.Default().PluginRoot()
	})

	// Phase 4: session
	session.Initialize(session.Options{CWD: appCwd})

	// Phase 5: hooks — depends on setting, session, llm, plugin
	hookSettings := setting.Default().Snapshot()
	plugin.MergePluginHooksIntoSettings(hookSettings)
	hook.Initialize(hook.Options{
		Settings:       hookSettings,
		SessionID:      session.Default().ID(),
		CWD:            appCwd,
		TranscriptPath: session.Default().TranscriptPath(),
		Completer:      buildHookCompleter(llm.Default().Provider()),
		ModelID:        llm.Default().ModelID(),
		EnvProvider:    plugin.PluginEnv,
	})

	return nil
}

func initExtensions(cwd string) {
	if err := plugin.Initialize(context.Background(), plugin.Options{CWD: cwd}); err != nil {
		log.Logger().Warn("Failed to initialize plugin", zap.Error(err))
	}
	skill.Initialize(skill.Options{CWD: cwd, PluginSkillPaths: pluginSkillPaths, SkillDirs: setting.Default().Snapshot().SkillDirs})
	persona.Initialize(cwd)
	command.Initialize(command.Options{
		CWD:                cwd,
		DynamicProviders:   []func() []command.Info{skillCommandInfos},
		PluginCommandPaths: pluginCommandPaths,
	})
	if err := subagent.Initialize(subagent.Options{CWD: cwd, PluginAgentPaths: pluginAgentPaths}); err != nil {
		log.Logger().Warn("Failed to initialize subagent", zap.Error(err))
	}
	if err := mcp.Initialize(mcp.Options{CWD: cwd, PluginServers: pluginMCPServers}); err != nil {
		log.Logger().Warn("Failed to initialize mcp", zap.Error(err))
	}
}

// discoverPlugins scans the working directory for plugins. Run on a cwd change
// so the new project's plugins (and their command / agent / MCP / hook
// contributions) replace the previous project's before reloadProjectServices
// rebuilds the project's feature services.
func discoverPlugins(cwd string) {
	if err := plugin.Initialize(context.Background(), plugin.Options{CWD: cwd}); err != nil {
		log.Logger().Warn("Failed to initialize plugin", zap.Error(err))
	}
}

// reloadProjectServices rebuilds the feature singletons that depend on the
// current project — its config dirs plus the active plugins' contributions —
// and re-points the services struct at the fresh instances. Both halves live
// here so the Initialize set and the Default()-regrab set cannot drift. The
// six: settings, skills, commands, subagents, MCP servers, personas. Plugins
// themselves are not rebuilt here — discoverPlugins does that on a cwd change,
// and a plugin load (--plugin-dir or /plugin install) has already done it.
func (m *model) reloadProjectServices(cwd string) {
	setting.Initialize(setting.Options{CWD: cwd})
	m.services.Setting = setting.Default()
	m.learnedStores.Update(cwd, m.services.Setting)

	skill.Initialize(skill.Options{CWD: cwd, PluginSkillPaths: pluginSkillPaths, SkillDirs: m.services.Setting.Snapshot().SkillDirs})
	m.services.Skill = skill.Default()

	command.Initialize(command.Options{
		CWD:                cwd,
		DynamicProviders:   []func() []command.Info{skillCommandInfos},
		PluginCommandPaths: pluginCommandPaths,
	})
	m.services.Command = command.Default()

	if err := subagent.Initialize(subagent.Options{CWD: cwd, PluginAgentPaths: pluginAgentPaths}); err != nil {
		log.Logger().Warn("Failed to initialize subagent", zap.Error(err))
	}
	m.services.Subagent = subagent.Default()

	if err := mcp.Initialize(mcp.Options{CWD: cwd, PluginServers: pluginMCPServers}); err != nil {
		log.Logger().Warn("Failed to initialize mcp", zap.Error(err))
	}
	m.services.MCP = mcp.DefaultRegistry()

	persona.Initialize(cwd)
	m.services.Persona = persona.Default()
}

func pluginCommandPaths() []command.PluginCommandPath {
	pPaths := plugin.GetPluginCommandPaths()
	paths := make([]command.PluginCommandPath, len(pPaths))
	for i, p := range pPaths {
		paths[i] = command.PluginCommandPath{
			Path:      p.Path,
			Namespace: p.Namespace,
			IsProject: p.Scope == plugin.ScopeProject || p.Scope == plugin.ScopeLocal,
		}
	}
	return paths
}

func pluginAgentPaths() []subagent.PluginAgentPath {
	pPaths := plugin.GetPluginAgentPaths()
	paths := make([]subagent.PluginAgentPath, len(pPaths))
	for i, p := range pPaths {
		paths[i] = subagent.PluginAgentPath{
			Path:      p.Path,
			Namespace: p.Namespace,
		}
	}
	return paths
}

func pluginSkillPaths() []skill.PluginSkillPath {
	pPaths := plugin.GetPluginSkillPaths()
	paths := make([]skill.PluginSkillPath, len(pPaths))
	for i, p := range pPaths {
		paths[i] = skill.PluginSkillPath{
			Path:      p.Path,
			Namespace: p.Namespace,
			IsProject: p.Scope == plugin.ScopeProject || p.Scope == plugin.ScopeLocal,
		}
	}
	return paths
}

func pluginMCPServers() []mcp.PluginServer {
	pServers := plugin.GetPluginMCPServers()
	servers := make([]mcp.PluginServer, len(pServers))
	for i, s := range pServers {
		servers[i] = mcp.PluginServer{
			Name:    s.Name,
			Type:    string(s.Config.Type),
			Command: s.Config.Command,
			Args:    append([]string(nil), s.Config.Args...),
			Env:     s.Config.Env,
			URL:     s.Config.URL,
			Headers: s.Config.Headers,
			Scope:   string(s.Scope),
		}
	}
	return servers
}

func commandSuggestionMatcher(cmdSvc *command.Registry) func(string) []suggest.Suggestion {
	return func(query string) []suggest.Suggestion {
		cmds := cmdSvc.GetMatching(query)
		result := make([]suggest.Suggestion, len(cmds))
		for i, c := range cmds {
			result[i] = suggest.Suggestion{Name: c.Name, Description: c.Description}
		}
		return result
	}
}

type agentRegistryAdapter struct {
	reg *subagent.Registry
}

func (a *agentRegistryAdapter) ListConfigs() []tool.AgentConfigInfo {
	configs := a.reg.ListConfigs()
	out := make([]tool.AgentConfigInfo, len(configs))
	for i, cfg := range configs {
		out[i] = subagent.ToAgentConfigInfo(cfg)
	}
	return out
}

func (a *agentRegistryAdapter) GetDisabledAt(userLevel bool) map[string]bool {
	return a.reg.GetDisabledAt(userLevel)
}

func (a *agentRegistryAdapter) SetEnabled(name string, enabled bool, userLevel bool) error {
	return a.reg.SetEnabled(name, enabled, userLevel)
}

func (a *agentRegistryAdapter) GetWriteEnabledAt(userLevel bool) map[string]bool {
	return a.reg.GetWriteEnabledAt(userLevel)
}

func (a *agentRegistryAdapter) SetWriteEnabled(name string, enabled bool, userLevel bool) error {
	return a.reg.SetWriteEnabled(name, enabled, userLevel)
}

func skillCommandInfos() []command.Info {
	return input.SkillCommandInfos()
}
