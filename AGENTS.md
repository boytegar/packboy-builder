# Packboy Builder Agent Guide

## Role
Repository-specific operating context for AI agents modifying this Go CLI/TUI. Evidence: `go.mod`, `cmd/pcb`, `internal/`, `docs/`, `Makefile`.

Read `.agents/` by task need, not wholesale:
- Start `./.agents/overview.md` when scope/orientation is unclear.
- Code conventions → `./.agents/style.md`.
- Visual/UI work → `./.agents/design.md`.
- Reusable packages/building blocks → `./.agents/components.md`.
- CLI commands, TUI dispatch, handlers, execution boundaries → `./.agents/routes.md`.
- Confirmed skills implementation artifacts → `./.agents/skills/`.
- Today's continuity → `./.agents/memory/{today}-memory.md`.

Read `.agents/` documentation once at session start; do not re-read unless files changed or scope shifts into previously unread material. Before any code add/edit/delete, read `./.agents/skills/` first. Skills generation was confirmed for this run; generated skills are referenced here. If unavailable/not applicable, continue and mark the gap `Needs verification`.

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
```text
Allowed folders/files: [type exact folder or file scope here]
``` 
If the scope field is filled by the user, treat it as a hard boundary; refuse edits outside it.

## RTK
RTK is detected at `/home/boytegar/.local/bin/rtk`; usability/capability version needs verification. Prefer RTK for token-efficient read/inspection where supported:
```bash
rtk ls .
rtk read <file>
rtk smart <file>
rtk find "<glob>" .
rtk grep "<pattern>" .
rtk diff <left> <right>
rtk err go build ./...
rtk go test ./...
```
These reduce tree, file, grep, diff, build-error, and Go-test output. Structured/data-analysis RTK workflows: Needs verification. If RTK is unavailable/not applicable, use normal repository tools. Do not invent RTK write/edit capabilities; none are evidenced.

## Workflow
Real commands/evidence:
```bash
go build ./...
GOCACHE=/tmp/gocache go test ./...
GOCACHE=/tmp/gocache go test ./tests/integration/cli/... ./tests/integration/session/...
gofmt -w <changed-go-files>
make <target>  # inspect Makefile for target names
```
Exact lint/format targets and environment constraints are documented in `.agents/overview.md`; commands not clearly defined are `Needs verification`.

## Structural/documentation rules
`.agents/` is living documentation. Structural add/rename/move/delete requires reviewing and updating `overview.md`, `style.md`, `design.md`, `components.md`, `routes.md`, `skills/` when enabled, and today's memory file. If a new area is uncovered, extend the relevant document. If stale, update it or report the gap. Today's memory uses `- [file] , [task] , [time]`; log meaningful completed work, decisions, blockers, and handoffs—not every exploratory action. Do not self-log a memory-file change.

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
Skills implementation was confirmed for this run. Generated artifacts live in `./.agents/skills/`; consult them before execution when the task requires them.

<!-- gortex:communities:start -->
<!-- gortex:skills:start -->
## Community Skills

| Area | Description | Skill |
|------|-------------|-------|
| Plugin 30 Dirs | 2972 symbols | `/gortex-plugin-30-dirs` |
| App Input 49 Dirs | 1876 symbols | `/gortex-app-input-49-dirs` |
| Mcp 34 Dirs | 1591 symbols | `/gortex-mcp-34-dirs` |
| App 22 Dirs | 1488 symbols | `/gortex-app-22-dirs` |
| Setting 42 Dirs | 1482 symbols | `/gortex-setting-42-dirs` |
| Hook 11 Dirs | 1129 symbols | `/gortex-hook-11-dirs` |
| App Conv 21 Dirs | 972 symbols | `/gortex-app-conv-21-dirs` |
| Setting 20 Dirs | 789 symbols | `/gortex-setting-20-dirs` |
| Tool 20 Dirs | 757 symbols | `/gortex-tool-20-dirs` |
| App Input 11 Dirs Providerselector | 749 symbols | `/gortex-app-input-11-dirs-providerselector` |
| App Conv 7 Dirs Render | 700 symbols | `/gortex-app-conv-7-dirs-render` |
| Search 24 Dirs | 698 symbols | `/gortex-search-24-dirs` |
| App Input 24 Dirs | 691 symbols | `/gortex-app-input-24-dirs` |
| Session 22 Dirs | 649 symbols | `/gortex-session-22-dirs` |
| Lsp 20 Dirs | 554 symbols | `/gortex-lsp-20-dirs` |
| Core 9 Dirs | 547 symbols | `/gortex-core-9-dirs` |
| Llm 5 Dirs | 439 symbols | `/gortex-llm-5-dirs` |
| App Input 16 Dirs | 410 symbols | `/gortex-app-input-16-dirs` |
| Task 9 Dirs | 384 symbols | `/gortex-task-9-dirs` |
| Session Transcript 1 Dirs Testeventsshape | 340 symbols | `/gortex-session-transcript-1-dirs-testeventsshape` |
<!-- gortex:skills:end -->

<!-- gortex:communities:end -->
