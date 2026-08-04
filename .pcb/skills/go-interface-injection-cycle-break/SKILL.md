---
name: go-interface-injection-cycle-break
description: Break Go import cycles by defining a narrow interface on the lower-level consumer and injecting the concrete higher-level dependency at wiring time.
origin: agent-created
---

# Break Go import cycles via interface injection

## When
A lower-level package (e.g. `internal/subagent`) needs to CALL into a higher-level
package (e.g. `internal/skill`) that would, if imported directly, create an import
cycle — commonly surfacing in tests (`subagent` test → `tool/registry` → `tool/fs`
→ `subagent`).

Two standard fixes; pick by call-site shape:
1. **App-layer wiring** (see `go-app-layer-side-effect-wiring` skill): move the
   call into the app layer (model methods). Best when the effect is a tool
   side-effect with an existing app-layer hook point (e.g. `applyToolSideEffects`).
2. **Interface injection** (this skill): keep the call in the lower package but
   behind a narrow interface, and inject the concrete at wiring time. Best when
   the lower package needs the higher one at runtime, not as a post-hook.

## Pattern
1. Define a minimal interface on the lower consumer naming only the method(s)
   it needs:
     // internal/subagent/executor.go
     type SkillMatcher interface { MatchForPrompt(text string) []string }
2. Add a setter that takes the interface:
     func (e *Executor) SetSkillMatcher(m SkillMatcher) { e.skillMatcher = m }
3. Call through the interface (nil-safe) at the point of need:
     if e.skillMatcher != nil {
         for _, b := range e.skillMatcher.MatchForPrompt(req.Prompt) { ... }
     }
4. Wire the concrete at the app composition root:
     // internal/app/agent.go (BuildParams)
     executor.SetSkillMatcher(m.services.Skill) // *skill.Registry satisfies it

## Why it works
The lower package depends on its own interface (inverted), never on the higher
package's concrete type. The concrete is supplied from the app layer, which may
import both. No cycle. The higher package needs zero knowledge of the lower.

## Gotchas
- Keep the interface narrow (one method, minimal args) so the higher package
  satisfies it implicitly — no `implements` ceremony in Go.
- Nil-guard the call site: the setter may not be called in tests/headless paths.
- Don't move the interface into a shared `internal/tool` "ports" package unless
  multiple lower consumers need it; a local interface keeps the dependency edge
  inside one file.
- Prefer app-layer wiring (#1) when the call is a tool side-effect — interface
  injection is for genuine runtime needs that must stay in the lower package.
