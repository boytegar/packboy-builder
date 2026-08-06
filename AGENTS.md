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

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes_tool` or `query_graph_tool` instead of Grep
- **Understanding impact**: `get_impact_radius_tool` instead of manually tracing imports
- **Code review**: `detect_changes_tool` + `get_review_context_tool` instead of reading entire files
- **Finding relationships**: `query_graph_tool` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview_tool` + `list_communities_tool`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### MCP tools sync before the agent opens

The harness never opens the agent alongside an MCP connect. When any MCP server finishes connecting, it **rebuilds the agent** so the model's toolset always includes that server's tools (`mcpToolsSignature` drift check). Tool schema sync completes first; the agent is built after — so third-party MCP tools (e.g. code-review-graph) are present and callable from the first turn, and the `researcher` subagent uses them (`query_graph_tool`, `semantic_search_nodes_tool`) instead of scanning files from zero.

### Key Tools

| Tool | Use when |
| ------ | ---------- |
| `detect_changes_tool` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context_tool` | Need source snippets for review — token-efficient |
| `get_impact_radius_tool` | Understanding blast radius of a change |
| `get_affected_flows_tool` | Finding which execution paths are impacted |
| `query_graph_tool` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes_tool` | Finding functions/classes by name or keyword |
| `get_architecture_overview_tool` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes_tool` for code review.
3. Use `get_affected_flows_tool` to understand impact.
4. Use `query_graph_tool` pattern="tests_for" to check coverage.
