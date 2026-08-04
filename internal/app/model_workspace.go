// Reactions to workspace changes: cwd switch (Bash `cd`), file-change
// notifications fed to hooks, project-context
// reload when cwd changes, persona reload when the user edits a persona file,
// and FileWatcher setup off the SessionStart hook outcome.
package app

import (
	"os"
	"path/filepath"

	"github.com/boytegar/packboy-builder/internal/app/trigger"
	"github.com/boytegar/packboy-builder/internal/confdir"
	"github.com/boytegar/packboy-builder/internal/hook"
	"github.com/boytegar/packboy-builder/internal/persona"
	"github.com/boytegar/packboy-builder/internal/subagent"
)

func (m *model) changeCwd(newCwd string) {
	if newCwd == "" || newCwd == m.env.CWD {
		return
	}
	oldCwd := m.env.CWD
	m.env.CWD = newCwd
	m.env.IsGit = m.services.Setting.IsGitRepo(newCwd)
	m.userInput.HandleCwdChange(newCwd)
	m.env.ClearCachedInstructions()
	m.refreshMemoryContext(newCwd, "cwd_changed")
	m.ReloadProjectContext(newCwd)
	m.ReconfigureAgentTool()
	m.services.Hook.SetCwd(newCwd)
	m.services.Hook.ExecuteAsync(hook.CwdChanged, hook.HookInput{OldCwd: oldCwd, NewCwd: newCwd})
}

func (m *model) fireFileChanged(filePath, source string) {
	if filePath == "" {
		return
	}
	m.services.Hook.ExecuteAsync(hook.FileChanged, hook.HookInput{FilePath: filePath, Source: source, Event: "change"})
}

func (m *model) ReloadProjectContext(cwd string) {
	// cwd changed: discover the new project's plugins, then rebuild the project
	// feature services that depend on them and re-point at them.
	discoverPlugins(cwd)
	m.reloadProjectServices(cwd)
	m.syncSettingsToHookEngine()
}

func (m *model) reloadPersonasIfChanged(filePath string) {
	if !persona.IsPersonaFile(m.env.CWD, filePath) {
		return
	}
	m.services.Persona.Reload()
	m.applyPersonaSkills()
	m.applyPersonaAgents()
	m.ReconfigureAgentTool()
}

// reloadAgentsIfChanged re-scans agent definition directories when a Write/Edit
// lands inside the user-level agents folder (~/.pcb/agents/). The registry
// overwrites by name, so re-calling LoadAgents is safe; the next AgentDirectory
// prompt section picks up new/changed definitions live. Best-effort: errors
// are logged but never surface to the tool result.
func (m *model) reloadAgentsIfChanged(filePath string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	agentsDir := filepath.Join(confdir.Dir(homeDir), "agents")
	rel, err := filepath.Rel(agentsDir, filePath)
	if err != nil || rel == "" || filepath.IsAbs(rel) {
		return
	}
	// Only reload for files under the agents dir (reject "../" escapes).
	for _, part := range filepath.SplitList(rel) {
		if part == ".." {
			return
		}
	}
	subagent.LoadAgents(m.env.CWD)
}

// enqueueMatchingSkills finds active skills whose name or description keywords
// appear in the user's prompt and enqueues their full instructions as a
// <system-reminder>. attachPendingReminders (called inside SubmitToAgent)
// drains the queue and appends the content to the user message. This gives
// the model the skill body without requiring an explicit Skill tool call —
// a conservative auto-trigger that complements the skills directory.
func (m *model) enqueueMatchingSkills(prompt string) {
	if m.services.Skill == nil {
		return
	}
	matches := m.services.Skill.MatchForPrompt(prompt)
	for _, body := range matches {
		m.services.Reminder.Enqueue(body)
	}
}

func (m *model) applyStartupHookOutcome(outcome hook.HookOutcome) {
	if outcome.InitialUserMessage != "" && m.env.InitialPrompt == "" && len(m.conv.Messages) == 0 {
		m.env.InitialPrompt = outcome.InitialUserMessage
	}
	if len(outcome.WatchPaths) == 0 {
		return
	}
	if m.systemInput.FileWatcher == nil {
		m.systemInput.FileWatcher = trigger.NewFileWatcher(m.services.Hook, func(outcome hook.HookOutcome) {
			if m.systemInput.AsyncHookQueue != nil && outcome.InitialUserMessage != "" {
				m.systemInput.AsyncHookQueue.Push(trigger.AsyncHookRewake{Notice: "File watcher hook triggered", Context: []string{outcome.InitialUserMessage}})
			}
		})
	}
	m.systemInput.FileWatcher.SetPaths(outcome.WatchPaths)
}
