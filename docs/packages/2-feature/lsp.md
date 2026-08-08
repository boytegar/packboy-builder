---
package: github.com/boytegar/packboy-builder/internal/lsp
layer: feature
---


Language Server Protocol (LSP) client — connects to language servers,
buffers their diagnostics, and powers the agent-facing `LSP` tool
(`internal/tool/lsp`). Transport is stdio with `Content-Length` JSON-RPC
framing (distinct from MCP's NDJSON transport).


LSP is how Packboy Builder gives the agent code intelligence — Go to
definition, find references, and read compiler/linter diagnostics —
without parsing the source itself. Servers are declared via the plugin
manifest `lspServers` block and start **lazily** on first use for a
matching file extension.

The package is deliberately protocol-shaped only: no editor glue. It
exposes a lazy `Manager` and a thin `Service` handle used by the app and
the tool adapter.


| Role | Shape | Consumers |
|---|---|---|
| **Manager** — server lifecycle + diagnostics cache | concrete `*Manager` | `Service`, tests |
| **Service** — app-facing handle + default wiring | `Default()/Initialize()/SetDefault()/ResetDefault()` | `internal/app`, `internal/tool/lsp` |
| **Transport** — Content-Length framing + subprocess lifecycle | `FrameReader`, `Client` | `Manager` |

`*Manager` is safe for concurrent use; `*Client` is single-goroutine and
owned by the manager. Diagnostics are cached per document URI, keyed on
server `publishDiagnostics` notifications.


Flow (lazy start on first request):

1. Tool calls `LSP` with an absolute file path.
2. `Manager.ServerForPath` picks the configured server by extension
   (`ExtensionToLanguage` keys).
3. `Manager.Start` launches the subprocess (if not already running),
   sends `initialize` + `initialized`, and stores the client.
4. The client sends `didOpen` (whole-document sync), then the requested
   method (`definition`, `references`), and reads the result.
5. `publishDiagnostics` notifications are captured into the URI-keyed
   cache and surfaced by the `diagnostics` action.

On app shutdown (or cwd reload) `Service.Shutdown`/`Initialize` close all
servers gracefully (`shutdown` → `exit` → SIGTERM → SIGKILL).

## Config

LSP servers are loaded from three sources, in priority order (later overrides
earlier for the same server name):

1. **JSON config files** (like MCP):
   - `~/.pcb/lsp.json` — user scope (global)
   - `./.pcb/lsp.json` — project scope (team shared)
   - `./.pcb/lsp.local.json` — local scope (personal, git-ignored)
2. **Plugin manifest** (`lspServers` / `.lsp.json` within a plugin)
3. **Built-in default catalog** (`catalog.go`, PATH-gated)

JSON file shape (`lsp.json`):

```json
{
  "lspServers": {
    "gopls": {
      "command": "gopls",
      "args": ["-mode=stdio"],
      "extensionToLanguage": {"go": "go", "mod": "go"},
      "disabled": false,
      "env": {},
      "initOptions": {}
    }
  }
}
```

Set `"disabled": true` to suppress a server from a lower-priority scope.

Plugin manifest (`lspServers`):

```json
{
  "lspServers": {
    "gopls": {
      "command": "gopls",
      "args": ["-mode=stdio"],
      "extensionToLanguage": { "go": "go", "mod": "go" }
    }
  }
}
```

Config is read via `plugin.GetPluginLSPServers()` and bridged into the
service in `internal/app/init.go`. Command/args are not shell-expanded
(see Security). Multiple plugin namespaces are keyed as
`<plugin>:<server>`.

## Security

- **No shell expansion.** Unlike Crush's crushrc/JSON (arbitrary `$(...)`
  execution at load), SAN does **not** shell-expand `command`/`args`.
  Config comes from plugins, which are already code the user opted into;
  this avoids adding a second arbitrary-code-execution surface.
- Server subprocesses inherit a detached process group and their stderr
  is drained into the debug log (never `os.Stderr`), so they cannot
  corrupt the Bubble Tea alt-screen or steal the terminal.

## Tests

- `frame_test.go` — Content-Length framing (embedded newlines, oversized
  bodies, missing headers, multiple messages, JSON round-trip).
- `manager_test.go` — server/extension routing, remembered-start-failure,
  diagnostics caching + sorting + copy semantics, service wiring.

## Future work / known gaps

- **Auto-start catalog** (Crush ships a bundled default server catalog;
  SAN requires explicit plugin config today).
- **Crash recovery** — a dead child currently fails in-flight requests;
  no auto-restart loop yet.
- **More capabilities** — hover, completion, document symbols, call
  hierarchy, rename, incremental sync, offset-encoding negotiation (files
  currently sync whole-document).
- **Diagnostics into read/edit tool output** — today diagnostics are
  pull-based via the `LSP` action, not auto-injected on file read.

## Local development

```bash
export GOCACHE=/tmp/pcb-go-build-cache
go test ./internal/lsp/ ./internal/tool/lsp/
```

## Phase 2 enhancements (shipped)

### Auto-injected diagnostics

`lsp.AppendDiagnostics(&result, filePath)` is called at the end of `Read`,
`Edit`, and `Write` tool execution. When an LSP server is configured for the
file's extension, cached diagnostics are appended to the tool's output as a
`--- LSP diagnostics ---` section. A background `didOpen` is fired so future
reads will have fresh diagnostics. This is non-blocking and best-effort —
diagnostics enrich the result but never gate it.

### Default server catalog

`internal/lsp/catalog.go` ships a minimal built-in catalog: gopls,
typescript-language-server, pyright, rust-analyzer, clangd,
lua-language-server, zls. `MergeWithDefaults` merges plugin-contributed
servers over defaults: plugin servers always win, and a default is included
only when its binary is on `exec.LookPath` and at least one of its extensions
is not already covered by a plugin. This means LSP works out-of-the-box when a
language server is installed, without any plugin config.

### Agent prompt visibility

The `LSP` tool's `Description()` already guides the model on when to use it
(prefer LSP for code intelligence when configured). The tool appears in the
model-facing schema list via `builtinToolOrder` whenever a server is
configured or a default catalog server is available on PATH.

## Phase 3 enhancements (shipped)

### Full tool action set

The `LSP` tool now supports 7 actions:

| Action | LSP method | Description |
|---|---|---|
| `diagnostics` | `publishDiagnostics` (cached) | Errors/warnings for a file |
| `definition` | `textDocument/definition` | Go-to-definition |
| `references` | `textDocument/references` | Find all references |
| `symbols` | `textDocument/documentSymbol` | Document symbol tree (hierarchical or flat) |
| `call_hierarchy` | `textDocument/prepareCallHierarchy` + `callHierarchy/incomingCalls`/`outgoingCalls` | Incoming/outgoing call graph |
| `rename` | `textDocument/rename` | Workspace-wide rename preview |
| `restart` | Manager.RestartServer | Force-restart the server for a language |

Symbol kinds are rendered as human-readable names (`Function`, `Struct`,
`Interface`, etc.) using the LSP SymbolKind enum.

### Crash recovery (auto-restart)

`Manager.Start` detects dead clients (`!client.IsAlive()`) and automatically
respawns them via `restartLocked`. The dead client is closed, the `started`
flag is reset, and `Start` re-enters to launch a fresh subprocess +
initialize handshake. This means a crashed language server (OOM, segfault)
is transparently recovered on the next request — no manual `restart` needed.

`RestartServer(name)` is also exposed for explicit restarts (the `restart`
tool action), which force-kills even a live server.

### Offset-encoding negotiation

The `initialize` request now advertises `positionEncodings: ["utf-8",
"utf-16", "utf-32"]`. The server's response `capabilities.positionEncoding`
is parsed and stored on the client (`setPositionEncoding`). The negotiated
encoding is queryable via `client.PositionEncoding()` (defaults to `utf-16`
per LSP spec). This enables correct position/range handling for servers
that prefer utf-8 (gopls) or utf-32.

### Tests

`phase3_test.go` adds 8 tests covering:
- Position encoding default + override
- RestartServer (unknown, not-started)
- Crash recovery (remembered failure)
- Catalog merge (empty, full cover, partial cover)
- Symbol kind name rendering
