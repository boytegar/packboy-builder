# CLI Entry & Startup Modes

## Overview

`pcb` supports several startup modes controlled by flags. The TUI is only launched in interactive mode; other modes produce plain stdout output.

| Flag | Behavior |
|------|----------|
| `pcb` | Launch interactive TUI |
| `pcb -p "prompt"` | Non-interactive: print response to stdout, no TUI |
| `pcb --plan "task"` | Start in plan mode (read-only) |
| `pcb -c` | Resume the most recent session |
| `pcb -r` | Pick a session from a list |
| `pcb -r <id>` | Resume a specific session directly |
| `pcb -c --fork` | Fork the most recent session |
| `pcb -r <id> --fork` | Fork a specific session |
| `pcb --plugin-dir PATH` | Load plugins from a directory |
| `pcb version` | Print version string |
| `pcb help` | Print help |

## UI Interactions

- **Interactive mode**: full TUI with input box, streaming output, and status bar.
- **Print mode (`-p`)**: no TUI; response is written to stdout line by line.
- **Plan mode**: status bar shows `[PLAN MODE]`; write tools are blocked.
- **Session resume (`-r`)**: a scrollable session picker is shown before the TUI starts.

## Automated Tests

```bash
GOCACHE=/tmp/gocache go test ./tests/integration/cli/... ./tests/integration/session/...
```

Covered:

```
TestVersionCommand                — pcb version prints version string without provider
TestHelpCommand                   — pcb help shows usage text
TestNonInteractivePrintMode       — -p writes response to stdout, no TUI
TestSessionFork_IsIndependent     — --fork creates independent session with ParentSessionID
TestSession_ContinueRestoresMessages — -c restores all messages in correct order
TestPlanMode_BlocksWriteTools     — --plan flag: write tools are blocked
TestPlanMode_AllowsReadTools      — --plan flag: read tools work normally
```

## Interactive Tests (tmux)

For transcript-specific startup validation, including `-c`, `-r`, `--fork`, and project transcript layout, see `docs/packages/session.md`.

```bash
tmux new-session -d -s t_cli -x 200 -y 50

# Test 1: Basic TUI startup
tmux send-keys -t t_cli 'pcb' Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: TUI appears with input box and status bar

# Test 2: Non-interactive print mode
tmux send-keys -t t_cli C-c
tmux send-keys -t t_cli 'pcb -p "what is 1+1"' Enter
sleep 5
tmux capture-pane -t t_cli -p
# Expected: "2" on stdout; no TUI launched

# Test 3: Plan mode startup
tmux send-keys -t t_cli 'pcb --plan "analyze this project"' Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: [PLAN MODE] visible in status bar
tmux send-keys -t t_cli C-c

# Test 4: Session resume picker
tmux send-keys -t t_cli 'pcb -r' Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: session list sorted by recency; navigate with arrow keys
tmux send-keys -t t_cli C-c

# Test 5: Continue latest session
tmux send-keys -t t_cli 'pcb -c' Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: latest session opens immediately with prior history
tmux send-keys -t t_cli C-c

# Test 6: Resume specific session by ID
PROJECT_DIR=~/.pcb/projects/$(pwd | sed 's#/#-#g')
SESSION_ID=$(find "${PROJECT_DIR}/transcripts" -name '*.jsonl' | head -1 | xargs basename | sed 's/\.jsonl$//')
tmux send-keys -t t_cli "pcb -r ${SESSION_ID}" Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: specified session loads directly without picker
tmux send-keys -t t_cli C-c

# Test 7: Version command (no provider needed)
tmux send-keys -t t_cli 'pcb version' Enter
sleep 1
tmux capture-pane -t t_cli -p
# Expected: version string printed to stdout

# Test 8: Help command
tmux send-keys -t t_cli 'pcb help' Enter
sleep 1
tmux capture-pane -t t_cli -p
# Expected: usage text with flags and subcommands

# Test 9: Plugin dir flag
mkdir -p /tmp/cli_plugin_test/.pcb-plugin
mkdir -p /tmp/cli_plugin_test/skills/hello
cat > /tmp/cli_plugin_test/.pcb-plugin/plugin.json << 'PEOF'
{"name": "cli-test", "version": "1.0.0", "description": "test"}
PEOF
cat > /tmp/cli_plugin_test/skills/hello/SKILL.md << 'PEOF'
---
name: hello
description: Say hello
allowed-tools: []
---
Say "plugin loaded" and nothing else.
PEOF
tmux send-keys -t t_cli 'pcb --plugin-dir /tmp/cli_plugin_test' Enter
sleep 2
tmux send-keys -t t_cli '/skills' Enter
sleep 1
tmux capture-pane -t t_cli -p
# Expected: "hello" skill listed from plugin
tmux send-keys -t t_cli C-c

# Test 10: Fork latest session
tmux send-keys -t t_cli 'pcb -c --fork' Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: new session with history from latest session

# Test 11: Fork specific session by ID
tmux send-keys -t t_cli C-c
tmux send-keys -t t_cli "pcb -r ${SESSION_ID} --fork" Enter
sleep 2
tmux capture-pane -t t_cli -p
# Expected: new session forked from the specified session

tmux kill-session -t t_cli
rm -rf /tmp/cli_plugin_test
```
