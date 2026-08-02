---
package: github.com/boytegar/packboy-builder/internal/skill
layer: feature
---

# skill

Loads markdown-defined skills from user / project / plugin scopes, tracks
their enable state, and renders the active-skills directory that the
harness attaches to user messages via the `skills-directory` reminder
(see [`concepts/harness-channels.md`](../../concepts/harness-channels.md)).

## Purpose

A skill is a markdown file (with YAML frontmatter) that the model can be
made aware of (`active`), made invocable via slash command (`enabled`), or
hidden (`disabled`). This package:

1. Discovers skills across the builtin, plugin, user, project, and
   Claude-compat scopes
   (`~/.claude/skills/`, `~/.pcb/plugins/*/skills/`, `~/.pcb/skills/`,
   `.claude/skills/`, `.pcb/plugins/*/skills/`, `.pcb/skills/`) with
   project overriding user overriding Claude-compat, plus any extra
   directories listed in `settings.json` `"skillDirs"` (see
   Configuration).
2. Persists per-skill state in user / project state stores.
3. Renders the active-skills block consumed by the `skills-directory`
   reminder provider in `internal/app`.

For the broader extension model see
[`concepts/extension-model.md`](../../concepts/extension-model.md). A
how-to-author-a-skill guide is tracked in `notes/tech-debt.md`.

## Contract

The package exposes `*Registry` directly. Skill consumers each use a
different subset of the registry surface — the TUI selector goes wide
(`List` / `GetStatesAt` / `SetState`), the slash-command flow uses
narrow lookups (`Get` / `FindByPartialName` / `GetSkillInvocationPrompt`),
the system-prompt builder uses one method (`PromptSection`), and the
session recorder attaches an observer (`SetStateChangeObserver`). No
shared narrow surface ⇒ no producer-side role interface earns its keep.

```go
package skill

// Registry is an opaque handle to the loaded skill set plus per-scope
// enabled-state stores. The type is exported so callers can hold and
// pass *Registry values; all fields are unexported.
type Registry struct { /* internal fields */ }

// Query
func (r *Registry) Get(name string) (*Skill, bool)
func (r *Registry) FindByPartialName(name string) *Skill
func (r *Registry) List() []*Skill
func (r *Registry) GetEnabled() []*Skill
func (r *Registry) GetActive() []*Skill
func (r *Registry) Count() int
func (r *Registry) IsEnabled(name string) bool

// State (used by the TUI selector)
func (r *Registry) SetState(name string, state SkillState, userLevel bool) error
func (r *Registry) GetStatesAt(userLevel bool) map[string]SkillState
func (r *Registry) SetEnabled(name string, enabled bool, userLevel bool) error
func (r *Registry) GetDisabledAt(userLevel bool) map[string]bool

// Rendering (consumed by the skills-directory reminder provider)
func (r *Registry) PromptSection() string
func (r *Registry) GetSkillInvocationPrompt(name string) string

// Recorder observer (used by the session recorder)
func (r *Registry) SetStateChangeObserver(cb StateChangeObserver)

// Package-level access
func Initialize(opts Options)
func Default() *Registry
func DefaultIfInit() *Registry        // nil if pre-Initialize
func SetDefaultRegistry(r *Registry)  // test-only
func ResetDefaultRegistry()           // test-only

// Skill, SkillState, SkillScope — see types.go for value types.
```

## Internals

- `Registry` (`registry.go`) is the only implementation, holding:
  - `skills []*Skill` — loaded by `loader`
  - `userStore`, `projectStore` — JSON-backed persistence of per-skill
    `SkillState`
  - `cwd` — for project-scope resolution
- `loader.go` walks the scopes in priority order, parsing
  `SKILL.md` frontmatter and bundled resource directories
  (`scripts/` / `references/` / `assets/`). Extra directories from
  `settings.json` `"skillDirs"` load at the `ScopeCustom` tier (above
  user, below project); each entry is `~`-expanded and resolved against
  cwd, and the merged user+project list is deduplicated.
- State (`disable` → `enable` → `active` → `disable`) cycles via
  `SkillState.NextState()`; the TUI's `/skill` flow uses this.
- Active skills render through `PromptSection()` and are delivered to
  the model via the `skills-directory` reminder attached to each user
  message; enable-only skills surface as slash commands but stay out of
  the model's awareness entirely.

## Lifecycle

- Construction: `Initialize(Options{CWD, PluginSkillPaths, SkillDirs})` at
  app startup, before `internal/command` builds its slash-command list.
  `SkillDirs` comes from `setting.Default().Snapshot().SkillDirs` and is
  re-resolved on a project reload. Singleton thereafter.
- Mutation: `SetEnabled` writes through to user or project store
  immediately; in-memory `skills` slice is updated in place.
- Plugin sources are added post-init by `internal/plugin` via
  `AddPluginSkills`.
- Concurrency: registry mutations are mutex-guarded; reads are
  RWMutex-locked.

## Configuration

`settings.json` (user `~/.pcb/settings.json` and project `.pcb/settings.json`,
merged as a deduplicated union) may list extra skill directories:

```json
{
  "skillDirs": ["/mnt/shared/skills", "~/team-skills"]
}
```

- Additive — the builtin, `~/.pcb/skills`, `.pcb/skills`, plugin, and
  Claude-compat directories are still scanned; this only extends the search.
- Entries are absolute paths or start with `~` (expanded to the user's home).
  Relative paths resolve against the project cwd.
- Project entries are appended after user entries during the settings merge,
  so a project-level `skillDirs` entry wins over a user-level one of the same
  skill name (both at the `ScopeCustom` tier).
- Skills loaded from `skillDirs` carry `ScopeCustom` — above `ScopeUser`, below
  `ScopeProject`, so `.pcb/skills/<name>` still overrides a `skillDirs` skill
  of the same name.

## Tests

```
internal/skill/skill_test.go            — loader, state cycling,
                                            scope priority, prompt rendering.
internal/skill/lazy_loading_test.go     — verifies content stays on disk
                                            until GetInstructions().
```

## See Also

- Code: `internal/skill/`
- Concepts: [`concepts/extension-model.md`](../../concepts/extension-model.md), [`concepts/harness-channels.md`](../../concepts/harness-channels.md)
- Related: [`packages/command.md`](command.md) (slash-command surface), [`packages/plugin.md`](plugin.md) (plugin-scoped skills), [`packages/reminder.md`](reminder.md) (the channel that delivers `PromptSection`)
- Layer: `feature` (see [`reference/dependency-rules.md`](../../reference/dependency-rules.md))
