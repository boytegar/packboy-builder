Token limits shape the context window every model runs against: the
auto-compaction trigger and the status bar's context percentage both need the
same denominator, so both resolve it through one resolver
(`llm.Store.EffectiveInputLimit` / `EffectiveInputLimitFor`).

## Role-scoped budgets via `/tokenlimit`

`/tokenlimit` opens an interactive selector: pick the context you are
budgeting — **Main agent**, **Sub-agent**, or **Sub-agent (write)** — then type
that context's input (context window) and output token limits. The values are
persisted to the **global** settings file (`~/.pcb/settings.json`) as
`mainTokenLimit` and `agentTokenLimit`, NOT per-model in `providers.json`:

```json
{
  "mainTokenLimit":  { "inputTokenLimit": 200000, "outputTokenLimit": 16000 },
  "agentTokenLimit": { "inputTokenLimit": 48000,  "outputTokenLimit": 4000  }
}
```

Entering `0 0` clears the override for the chosen role. Both limits apply to
every model in every provider — the main agent and its subagents get separate
denominations, so a tight subagent window cannot be widened by a main budget.

### Resolution priority

For the main agent (status bar + main compaction):

1. `mainTokenLimit.inputTokenLimit` in `~/.pcb/settings.json`
2. `tokenLimit` in `~/.pcb/settings.json` (legacy global override)
3. `PCB_INPUT_LIMIT` env var
4. Per-model `tokenLimits` in `providers.json`
5. Model cache (provider API)
6. `0` = unknown (compaction skipped, bar shows `--`)

For subagents (their compaction trigger uses the same chain but starts at
`agentTokenLimit.inputTokenLimit`). Output caps follow the same role split
(`mainTokenLimit.outputTokenLimit` / `agentTokenLimit.outputTokenLimit`) and
fall back to the provider's cached output limit, then 8192.

Auto-compaction fires when the prompt reaches 95% of the role's input limit
(`core.AutoCompactThresholdPercent`). The status bar's critical tier derives
from the same constant so the bar turns critical exactly when compaction is
due. See `internal/core/message.go` (`NeedsCompaction`).

## Where limits come from otherwise

| Source | Location | Description |
|--------|----------|-------------|
| Role budget | `~/.pcb/settings.json` `mainTokenLimit` / `agentTokenLimit` | Set via `/tokenlimit`; **highest priority** |
| Global override | `~/.pcb/settings.json` `tokenLimit` | Legacy single-window override |
| Env | `PCB_INPUT_LIMIT` | Window for a model Packboy Builder cannot size |
| Token Limits | `providers.json` `tokenLimits → {modelID}` | Manual per-model override |
| Model Cache | `providers.json` `models → {provider:auth} → models[]` | From provider's `ListModels()` API (e.g., Gemini) |

| Command | Action |
|---------|--------|
| `/tokenlimit` | Open the selector: pick Main agent / Sub-agent / Sub-agent (write), enter input + output limits, saved to global settings |

```
/tokenlimit
    │
    ▼
┌───────────────────────────────────┐
│ Token Limits                      │
│   [ ] Main agent        in 200K · out 16K   │
│   [ ] Sub-agent         in 48K  · out 4K    │
│   [ ] Sub-agent (write) in 48K  · out 4K    │
└───────────────────────────────────┘
    │  Enter on a row
    ▼
┌───────────────────────────────────┐
│ Sub-agent — limits for read-only  │
│ sub-agents.                       │
│   ▎Input tokens   48000           │
│    Output tokens  4000            │
└───────────────────────────────────┘
    │  Tab/↑↓ switch field · Enter save · Esc back
    ▼
persisted to ~/.pcb/settings.json → runtime resolver (compaction + status bar)
```

The old behaviour — auto-fetching the current model's limits with an isolated
web-search agent and saving per-model overrides to `providers.json` — has been
removed. Limits resolve from the provider's model cache when no override is
set.

| File | Purpose |
|------|---------|
| `internal/app/input/on_token_limit_selector.go` | `/tokenlimit` selector overlay (target picker + numeric editor) |
| `internal/setting/loader.go` | `UpdateTokenLimitFor` — persist `mainTokenLimit` / `agentTokenLimit` to global settings |
| `internal/setting/settings.go` | `MainTokenLimit` / `AgentTokenLimit` fields + `TokenLimitOverride` type |
| `internal/llm/inputlimit.go` | `EffectiveInputLimitFor` role-scoped resolver, `TokenRole` |
| `internal/llm/llm.go` | `Client.InputLimit()` / `effectiveMaxTokens()` role-aware |
| `internal/subagent/executor.go` | subagent clients built with `TokenRoleAgent` |
| `internal/app/kit/token.go` | token usage rendering and context-percentage indicator |

## Status display

When context usage reaches 80%+ of the role's input limit:

```
⚡ 180K/200K (90%)
```

The percentage's denominator is `EffectiveInputLimitFor` — the same window the
compaction trigger fires on, so the bar can never fill against a different
window than the one compaction uses.

## See Also

- [`slash-commands.md`](slash-commands.md) — command reference
- [`cost-tracking.md`](cost-tracking.md) — token accounting across sessions
