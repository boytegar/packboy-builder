# Packboy Builder Agent Guide

Repository-specific operating context for AI agents modifying this Go CLI/TUI. Evidence: `go.mod`, `cmd/pcb`, `internal/`, `docs/`, `Makefile`.

Read `.agents/` by task need, not wholesale:
- `./.agents/overview.md` — scope/orientation
- `./.agents/style.md` — code conventions
- `./.agents/design.md` — visual/UI work
- `./.agents/components.md` — reusable building blocks
- `./.agents/routes.md` — CLI commands, TUI dispatch, handlers, execution boundaries
- `./.agents/skills/` — confirmed skills implementation artifacts
- `./.agents/memory/{today}-memory.md` — today's continuity

Read once at session start; do not re-read unless files changed or scope shifts. Before any code add/edit/delete, read `./.agents/skills/` first. If unavailable, continue and mark the gap `Needs verification`.

## Operating rules
- Evidence first; do not invent stack, commands, boundaries, or conventions.
- Large or parallelizable scope → swarm agents; choose agent count from dependency graph.
- Repository discovery/search/non-trivial investigation → researcher subagent swarm before single-agent work, except trivially local work.
- Use TDD for application logic changes: failing test → minimal implementation → relevant tests pass. Docs-only/mechanical edits do not require TDD.
- Never run `git push` or any remote push.
- Work only inside repository scope documented here; do not modify outside `/mnt/shared/Project/Tools/san`.
- Read `docs/reference/dependency-rules.md` and `docs/design/principles.md` before internal package changes.
- Keep `internal/core` dependency-light; preserve `cmd → app → feature → core/infrastructure` direction.
- Generated/derived files: `bin/`, build outputs, and generated skill artifacts are not hand-edited unless their workflow explicitly requires it.

## Hard folder scope
If a scope field is filled by the user, treat it as a hard boundary; refuse edits outside it.

## Workflow
```bash
go build ./...
GOCACHE=/tmp/gocache go test ./...
GOCACHE=/tmp/gocache go test ./tests/integration/cli/... ./tests/integration/session/...
gofmt -w <changed-go-files>
make <target>  # inspect Makefile for target names
```
Exact lint/format targets and environment constraints are documented in `.agents/overview.md`; commands not clearly defined are `Needs verification`.

## Structural/documentation rules
`.agents/` is living documentation. Structural add/rename/move/delete requires reviewing and updating `overview.md`, `style.md`, `design.md`, `components.md`, `routes.md`, `skills/` when enabled, and today's memory file. Today's memory uses `- [file] , [task] , [time]`; log meaningful completed work, decisions, blockers, and handoffs—not every exploratory action. Do not self-log a memory-file change.

## Completion checklist
- Correct file placement; reuse existing building blocks.
- Dependency direction preserved; tests/verification run where gates exist.
- Entrypoint/route/execution-flow docs updated after boundary changes.
- `.agents/` refreshed after structural changes.
- Today's memory log updated for meaningful work.
- Context remains aligned with repository state.

## Definition of Done
Relevant repository verification gates pass and are evidenced (Go build, relevant Go tests, integration tests, formatting, and any detected lint gates). A change is incomplete when an applicable existing gate fails or remains unrun without an explicit report.

## Skills
Skills are lazy-loaded via the Skill tool when a task matches (SCAN/MATCH protocol). They are NOT auto-injected into context. To list available skills, use the Skill tool.
