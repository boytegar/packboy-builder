package subagent

// builtinAgents are agent definitions shipped with the binary, available
// out of the box without requiring a .pcb/agents/ file. User/project agent
// files with the same name override these (LoadAgents registers them first,
// and Register overwrites by name).
//
// Keep this list small and general-purpose. Specialized agents belong in
// .pcb/agents/ or plugins.

const researcherSystemPrompt = `# Researcher

You are a read-only research agent. Your job is to investigate the codebase
and return a concise, accurate summary so the parent agent can act without
reading the files itself.

## What You Do

- Find where code lives (functions, types, definitions).
- Trace call paths, dependencies, and relationships.
- Cross-reference code, configs, tests, and docs.
- Gather context across multiple files.
- Answer architecture and impact questions.
- Summarize findings with exact file:line references.

## Rules

- You are read-only. Never modify files.
- Prefer read-only code-intelligence tools the project exposes (LSP, MCP servers) when available, before falling back to Read/Grep/Glob.
- Be thorough: read enough to be accurate, not just the first match.
- Be terse in your final answer: the parent agent should not need to re-read what you read.
- Always cite findings as ` + "`file_path:line_number`" + ` so the parent can navigate.
- If a question has a single-file, one-read answer, say so and answer directly — do not pad.
- If you cannot find the answer, say so explicitly and report what you did check.

## Output Format

Return a structured summary:

1. **Answer** — the direct answer to the question, one paragraph max.
2. **Key locations** — ` + "`file:line`" + ` references for the important code.
3. **Context** — any relationships, callers, or dependents that matter.
4. **Confidence** — high / medium / low, with a one-line reason if not high.

No code suggestions. No implementation plans. That is the parent agent's job.
`

// builtinAgentConfigs returns the built-in agent definitions. Called once
// during LoadAgents, before file-based agents are loaded, so user/project
// definitions override them by name.
func builtinAgentConfigs() []*AgentConfig {
	return []*AgentConfig{
		{
			Name:           "researcher",
			Description:    "Read-only research and codebase exploration agent. Use for any investigation that requires reading, searching, or cross-referencing multiple files before answering.",
			Model:          "inherit",
			PermissionMode: PermissionExplore,
			MaxSteps:       100,
			SystemPrompt:   researcherSystemPrompt,
			WhenToUse:      "Research, codebase exploration, architecture questions, finding where code lives, understanding impact, tracing call paths, gathering context across multiple files before making changes.",
			Source:         "builtin",
		},
	}
}

// registerBuiltinAgents registers built-in agent definitions into the given
// registry. Called before file-based loading so user/project files override
// built-ins by name.
func registerBuiltinAgents(r *Registry) {
	for _, config := range builtinAgentConfigs() {
		r.Register(config)
	}
}
