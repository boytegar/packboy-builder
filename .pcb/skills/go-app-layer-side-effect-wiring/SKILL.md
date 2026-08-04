---
name: go-app-layer-side-effect-wiring
description: Wire tool-executor side-effects at the app layer to avoid Go import cycles when the effect touches a higher-level package.
origin: agent-created
---

# Go: Wire Tool Side-Effects at the App Layer, Not in the Executor

## When to use
A tool executor (`internal/tool/<pkg>`) needs to trigger a side-effect that lives in a
higher-level package (`internal/subagent`, `internal/persona`, `internal/skill`, …).
Directly importing the higher package from the executor **breaks** because the
higher package's tests transitively import `internal/tool` (e.g. `subagent` tests
import `internal/tool/registry` → `internal/tool/fs`), closing an import cycle:

```
internal/tool/fs  ──imports──▶  internal/subagent
internal/subagent (_test) ──imports──▶  internal/tool/registry ──▶ internal/tool/fs
```

`go build ./...` succeeds; `go test ./internal/<higher>/` fails with
`import cycle not allowed in test`.

## The fix
Do NOT add the import or the logic to the tool executor. Route the side-effect
through the **app layer** that already bridges tools and registries.

In this repo the existing seam is `internal/app/model_tool_effects.go`
`applyToolSideEffects(toolName, sideEffect)`. It already switches on tool name and
reads `resp["filePath"]` from the tool's `HookResponse` payload. Add a new
`model` method next to the existing reloaders and call it from the matching case:

```go
// internal/app/model_tool_effects.go
case "Write", "Edit":
    if filePath := kit.MapString(resp, "filePath"); filePath != "" {
        m.fireFileChanged(filePath, toolName)
        m.reloadPersonasIfChanged(filePath)
        m.reloadAgentsIfChanged(filePath)   // ← new
        if m.env.FileCache != nil { m.env.FileCache.Touch(filePath) }
    }
```

```go
// internal/app/model_workspace.go (app package already imports the higher pkg)
func (m *model) reloadAgentsIfChanged(filePath string) {
    // match a path prefix, then call the higher pkg's public re-scan
    subagent.LoadAgents(m.env.CWD)
}
```

## Why this is the right layer
- `internal/app` is allowed to import both `internal/tool` and `internal/subagent`
  (it's the top of the dependency graph; see `docs/reference/dependency-rules.md`).
- `applyToolSideEffects` runs synchronously in the same turn, so the reloaded
  state is visible to the **next** prompt build (e.g. `AgentDirectory` is a live
  `func()` reading the mutable registry, not a cached snapshot).
- Keeps tool executors dependency-free and testable in isolation.

## Anti-pattern to avoid
Adding `import ".../internal/subagent"` inside `internal/tool/fs/write.go` "works"
for production builds but silently breaks the higher package's test compile.
Always check: **does the target package (or its tests) transitively import the
tool package I'm editing?** If yes → app-layer callback, not a direct import.

## Checklist
1. Tool produces a `HookResponse`/sideEffect map with the needed data (filePath, etc.).
2. App's `applyToolSideEffects` already deserializes it — add a `model` method.
3. Put the method in a workspace/effects file (`model_workspace.go`) where
   `internal/app` already imports the higher package.
4. Gate on a path prefix so unrelated writes are no-ops.
5. Verify with `go test ./internal/<higher>/` — the cycle failure is the signal.
