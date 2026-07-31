# Getting Started

A 5-minute path from install to first agent turn.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/boytegar/packboy-builder/main/install.sh | bash
```

Re-run the same command to upgrade. To uninstall, append `-s uninstall`.

Alternatives:

```bash
# via Go toolchain
go install github.com/boytegar/packboy-builder/cmd/pcb@latest

# from source
git clone https://github.com/boytegar/packboy-builder.git
cd pcb && go build -o pcb ./cmd/pcb
```

The binary is a single ~12 MB Go executable; no Node, no runtime.

## First Run

```bash
pcb
```

On first launch, Packboy Builder drops into the TUI. Type `/models` to connect a
provider — you will be asked for an API key (or routed through Vertex AI
for Anthropic). Supported providers and the env var each one reads:

| Provider | Variable |
|---|---|
| Anthropic | `ANTHROPIC_API_KEY` (or Vertex AI) |
| OpenAI | `OPENAI_API_KEY` |
| Google | `GOOGLE_API_KEY` |
| Moonshot | `MOONSHOT_API_KEY` |
| Alibaba | `DASHSCOPE_API_KEY` |
| MiniMax | `MINIMAX_API_KEY` |
| MiMo | `MIMO_API_KEY` |
| Z.ai (GLM / GLM Coding Plan) | `BIGMODEL_API_KEY` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| Ollama (local) | `OLLAMA_BASE_URL` (default `http://localhost:11434/v1`) |
| SenseNova | `SENSENOVA_API_KEY` |
| Agnes-AI | `AGNESAI_API_KEY` |

You can also set them in `.env` or `~/.pcb/providers.json`.

## First Turn

Type a prompt and press `Enter`:

```
> explain what this repo does
```

Packboy Builder reads your project, plans, and acts. Tool calls (file reads,
edits, bash) trigger a permission prompt by default — press `Y` to
approve once, `A` to approve-all for this session.

## Cheat Sheet

| Action | Key / command |
|---|---|
| Approve pending tool call | `Y` |
| Approve all pending of this kind | `A` |
| Reject pending tool call | `N` |
| Toggle permission mode | `Shift+Tab` |
| Bypass permissions (auto-accept all) | `/yolo` (or `/yolo on` / `/yolo off`) |
| Expand tool details | `Ctrl+O` |
| Cancel in-flight turn | `Ctrl+C` |
| Exit | `Ctrl+D` or `/exit` |
| List all slash commands | `/help` |
| Switch model | `/models` |
| Switch persona | `/persona` |
| Save / resume session | `pcb --continue`, `pcb --resume` |

## One-Shot and Piped Modes

```bash
pcb "explain this function"          # one-shot, prints answer and exits
cat main.go | pcb "review"           # piped input
pcb --continue                       # resume the last session
```

## Where Configuration Lives

| Scope | Path | What it holds |
|---|---|---|
| User | `~/.pcb/providers.json` | Provider connections, current model |
| User | `~/.pcb/settings.json` | Permissions, hooks, env, persona, search provider |
| User | `~/.pcb/skills/` `~/.pcb/agents/` `~/.pcb/commands/` `~/.pcb/plugins/` | Your personal extensions |
| Project | `<project>/.pcb/settings.json` | Per-project overrides |
| Project | `<project>/.pcb/{skills,agents,commands}/` | Project-scoped extensions |
| Project | `<project>/AGENTS.md` or `CLAUDE.md` | Auto-loaded into the agent prompt |

See [`reference/configuration.md`](../reference/configuration.md) for the
full schema.

## What to Read Next

- [Writing a skill](writing-a-skill.md) — your first user extension.
- [Writing a subagent](writing-a-subagent.md) — define a parallel agent.
- [Writing a plugin](writing-a-plugin.md) — bundle skills + agents + commands.
- [`docs/concepts/architecture.md`](../concepts/architecture.md) — how the system is built.
- [`reference/slash-commands.md`](../reference/slash-commands.md) —
  every `/command`.
