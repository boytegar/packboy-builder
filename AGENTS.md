# Packboy Builder Agent Guide

This file is the short navigation map for agents and contributors. Keep durable
knowledge in `docs/`; keep this file focused on where to look and what rules to
follow before changing code.

`AGENTS.md` and `CLAUDE.md` at the project root are loaded into the running
agent's prompt at startup (runtime instruction files). Prefer `AGENTS.md`
for Packboy Builder-native project guidance; `CLAUDE.md` remains for Claude Code
compat. `SAN.md` is not loaded.

## Start Here

- Product overview: `README.md`
- Documentation index: `docs/index.md`
- Detailed architecture: `docs/concepts/architecture.md`
- Package map and ownership: `docs/reference/package-map.md`
- Dependency rules: `docs/reference/dependency-rules.md`
- Feature notes: `docs/packages/index.md`
- Development workflow: `docs/operations/development.md`

## Repository Shape

- `cmd/pcb`: CLI entrypoint and command wiring.
- `internal/app`: Bubble Tea TUI shell, model composition, event routing.
- `internal/core`: stable agent, message, tool, and system-prompt contracts.
- `internal/agent`: agent construction and session-facing runtime setup.
- `internal/llm`: model provider registry, clients, cost and logging helpers.
- `internal/tool`: built-in tool registry, schemas, adapters, and executors.
- `internal/session`: transcript persistence, projection, metadata, resume.
- `internal/task`, `internal/subagent`, `internal/cron`: background work and orchestration.
- `internal/command`, `internal/skill`, `internal/plugin`, `internal/mcp`, `internal/hook`: extension surfaces.
- `internal/setting`, `internal/persona`: configuration and persona overlays.
- `internal/log`, `internal/secret`, `internal/filecache`, `internal/markdown`, `internal/confdir`: infrastructure helpers.
- `internal/selflearn`: background memory and skill review loop.
- `docs`: durable explanations, design decisions, operations, and references.

## Rules

Before editing internal packages, read:

- `docs/reference/dependency-rules.md` — allowed import directions and the
  rule for each layer.
- `docs/design/principles.md` — coding principles for package structure,
  interfaces, tests, and context handling.

Update those files when the rules change. Do not duplicate them here.

## Subagent Usage

**Research and exploration must use the `researcher` subagent** (mode=explore),
not the main agent's own Read/Grep/Glob tools, when the task involves:

- Reading or searching more than 1-2 files.
- Tracing call paths, dependencies, or impact across the codebase.
- Answering architecture or "where does X live" questions.
- Gathering context before making changes.

The main agent should call the `Agent` tool with `name="researcher"` and a
self-contained `prompt` describing what to find. The subagent runs with its own
isolated context and returns a concise summary with `file:line` references,
keeping the main agent's context window clean.

**Exception**: a single, direct Read/Grep/Glob call is fine when the exact file
and target are already known (e.g. reading one function to edit it). When in
doubt, delegate to the researcher subagent.

## Common Commands

See `docs/operations/development.md` for build / test / lint / format
and the sandbox-friendly `GOCACHE` workaround. Update that file when
commands change. Do not duplicate them here.

## Documentation Rules

- Add or update docs in the same change as architecture or workflow changes.
- Each feature document should list purpose, entrypoints, core packages, flow,
  configuration, tests, and common pitfalls.
- Architecture decision records live in `docs/design/decisions/`.
- File naming rules live in `docs/reference/file-naming.md`.
- Active plans live in `notes/active/`; completed plans move to
  `notes/completed/`.



<!-- gortex:communities:start -->
<!-- gortex:skills:start -->
## Community Skills

| Area | Description | Skill |
|------|-------------|-------|
| Plugin 30 Dirs | 3033 symbols | `/gortex-plugin-30-dirs` |
| App Input 49 Dirs | 1871 symbols | `/gortex-app-input-49-dirs` |
| Mcp 34 Dirs | 1620 symbols | `/gortex-mcp-34-dirs` |
| Setting 44 Dirs | 1582 symbols | `/gortex-setting-44-dirs` |
| App 22 Dirs | 1488 symbols | `/gortex-app-22-dirs` |
| Hook 11 Dirs | 1129 symbols | `/gortex-hook-11-dirs` |
| App Conv 21 Dirs | 953 symbols | `/gortex-app-conv-21-dirs` |
| Setting 21 Dirs | 810 symbols | `/gortex-setting-21-dirs` |
| App Input 11 Dirs | 763 symbols | `/gortex-app-input-11-dirs` |
| Tool 20 Dirs | 751 symbols | `/gortex-tool-20-dirs` |
| Search 24 Dirs | 698 symbols | `/gortex-search-24-dirs` |
| App Conv 6 Dirs Render | 668 symbols | `/gortex-app-conv-6-dirs-render` |
| Session 22 Dirs | 649 symbols | `/gortex-session-22-dirs` |
| Lsp 20 Dirs | 554 symbols | `/gortex-lsp-20-dirs` |
| Core 8 Dirs Thinkact | 518 symbols | `/gortex-core-8-dirs-thinkact` |
| App Input 19 Dirs | 459 symbols | `/gortex-app-input-19-dirs` |
| Llm 5 Dirs | 439 symbols | `/gortex-llm-5-dirs` |
| Todo 6 Dirs | 423 symbols | `/gortex-todo-6-dirs` |
| App Input 16 Dirs | 410 symbols | `/gortex-app-input-16-dirs` |
| Session Transcript 1 Dirs Testeventsshape | 340 symbols | `/gortex-session-transcript-1-dirs-testeventsshape` |
<!-- gortex:skills:end -->

<!-- gortex:communities:end -->
