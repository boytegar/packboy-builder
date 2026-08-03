# Plan: Integrate Maestro (CLI + UI modes) into San

## Goal
Add a Maestro integration to san so the user can drive the Maestro multi-agent
orchestration daemon from **both** surfaces:
1. **CLI mode** — a new `pcb maestro` cobra subcommand that talks to the local
   Maestro daemon over HTTP (127.0.0.1:3001).
2. **UI mode** — a **san-served HTMX web UI** (localhost web server that returns
   HTML fragments and proxies to the Maestro daemon), launched via a new
   `pcb maestro ui` cobra subcommand and a `/maestro` TUI slash command. The
   browser opens to the san-hosted page; htmx drives partial updates without a
   JS SPA framework. **No Electron.**

Maestro = a local Go daemon that supervises parallel AI coding-agent sessions in
isolated git worktrees, with a thin cobra CLI and an Electron UI as its control
surfaces. San already has a cobra CLI, a bubbletea TUI, and an existing
localhost-web-server blueprint (`pcb inspector`) — this plan reuses all of
those. The Maestro daemon must already be running (`pcb maestro start`) for
product commands; the HTMX UI proxies to it over loopback.

User decisions (confirmed):
- Integration form: **HTTP client directly to the daemon** (127.0.0.1:3001).
- "UI mode" = **san-served HTMX web UI** (NOT Electron). Server returns HTML
  fragments; htmx swaps them in. Browser launched like `pcb inspector`.
- Command scope: **All** Maestro commands (daemon control + project + agent + spawn + session + send).
- `to` binary: **already installed, assumed on PATH** — no install flow needed.
- UI tech: **HTMX** (san serves HTML fragments over loopback; no Electron).

## Knowledge summary

### Scope — verified
- San CLI entrypoint: `cmd/pcb/main.go:93` (`var rootCmd`), flat subcommands
  registered in `init()` via `rootCmd.AddCommand(...)` at `cmd/pcb/main.go:71-76`.
  Existing subcommands: `version` (`:161`), `help`, `mcp`, `update`.
  San currently uses **flat top-level subcommands only** (no nested
  `cmd.AddCommand` on a non-root command). Shape example `versionCmd` `:161-186`
  (`Use`/`Short`/`Run` + flags).
- San TUI: bubbletea model `internal/app/model.go:40` (`type model struct`),
  `Update` at `internal/app/update.go:96`, `View` at `internal/app/view.go:21`.
  Slash commands are the primary TUI action surface; builtin handlers registered
  in `builtinCommandHandlers()` map at `internal/app/input/slash_command.go:106-137`.
  Handler signature: `func (c *SlashCommandController) handleX(ctx context.Context, args string) (string, tea.Cmd, error)` (seen at `:142`).
- OS-launch helper exists: `openBrowser(url)` at `cmd/pcb/inspector.go:118-132`
  → `exec.Command("open"/"xdg-open"/"rundll32...").Start()`. Loopback-only guard
  `requireLoopback` at `:105`. Detach helper `proc.DetachSession` at
  `internal/proc/proc_unix.go` (used by `internal/tool/fs/bash.go`).
- **HTMX/UI blueprint exists**: `pcb inspector` (`cmd/pcb/inspector.go:35-92`) =
  cobra subcommand → `net.Listen("tcp","127.0.0.1:0")` → `http.Server` with
  `inspector.New(projectDir).Handler()` (`internal/inspector/server.go`) → embed
  assets via `internal/inspector/ui/embed.go` (`embed.FS`) served at `/`, with
  `internal/inspector/ui/assets/{index.html,style.css,app.js}`. Plain JS today;
  app.js fetches `/api/sessions`. Maestro UI = same shape but htmx-driven
  (server returns HTML fragments, no SPA JS).
- Settings struct: `internal/setting/settings.go:27` (`type Data struct`); new
  config field = add field + JSON tag, read via `setting.Default().Data.<Field>`.
- Docs convention: feature docs under `docs/packages/` (tiered folders), template
  at `docs/packages/TEMPLATE.md` (per `docs/packages/index.md`).

### Entry points — verified
- New CLI entry: add `maestroCmd` to `rootCmd.AddCommand(...)` in `init()`
  at `cmd/pcb/main.go:71-76`. New file `cmd/pcb/maestro.go` for the subcommand
  + its own sub-subcommands (san's first nested subcommand tree).
- New TUI entry: add `"maestro": (*SlashCommandController).handleMaestroCommand`
  to the map at `internal/app/input/slash_command.go:107-136`.
- New HTTP client pkg: `internal/maestro/` — `client.go` (thin REST client to
  the daemon), `types.go` (request/response structs), `commands.go` (one fn
  per daemon route).
- New HTMX UI pkg: `internal/maestro/ui/` — `server.go` (http.ServeMux
  returning HTML fragments, proxies to `maestro.Client`), `embed.go` (`embed.FS`),
  `assets/{index.html,style.css,app.js}` (htmx + hyperscript minimal). Mirrors
  `internal/inspector/{server.go,ui/embed.go,ui/assets/}`.

### Dependencies — verified
- HTTP: **no shared generic HTTP helper** in san — LLM/search/mcp each use
  `net/http` directly (seen across `internal/llm/*`, `internal/search/*`,
  `internal/mcp/transport/http.go`). Plan: use `net/http` directly in
  `internal/maestro/client.go`, mirroring e.g. `internal/search/exa.go` style.
- Daemon routes (from `docs/cli/README.md`):
  - `GET/POST /api/v1/projects`, `GET/PUT/DELETE /api/v1/projects/{id}` & `/config`
  - `GET /api/v1/agents` (`--refresh` → `POST /api/v1/agents/refresh`)
  - `POST /api/v1/sessions` (spawn), `GET /api/v1/sessions`, `GET /api/v1/sessions/{id}`
  - `POST /api/v1/sessions/{id}/kill`, `/restore`, `PATCH /sessions/{id}` (rename),
    `POST /api/v1/sessions/cleanup`, `/sessions/{id}/pr/claim`
  - `GET /api/v1/orchestrators`
  - `POST /api/v1/sessions/{id}/send`, `/sessions/{id}/preview`
  - `POST /api/v1/sessions/{id}/activity` (hidden hooks)
  - `/healthz`, `/readyz`, `/shutdown`, `running.json` handshake.
- Config env vars to respect: `MAESTRO_PORT` (def 3001), `MAESTRO_RUN_FILE`,
  `MAESTRO_DATA_DIR`, `MAESTRO_REQUEST_TIMEOUT` (def 60s), `MAESTRO_SHUTDOWN_TIMEOUT`.
- Electron launch removed. San now **owns** the UI: `pcb maestro ui` boots a
  loopback web server (htmx-rendered) that proxies to the Maestro daemon.
  Routes hit the daemon via `internal/maestro.Client`. Reuses `openBrowser`
  from `cmd/pcb/inspector.go:118` + `requireLoopback` guard `:105`.

### Conventions — verified / inferred
- San layers: `cmd/pcb` → `internal/app` → `internal/tool`/`internal/subagent`/
  `internal/setting`. New `internal/maestro` is an infrastructure helper pkg
  (like `internal/search`) — importable by both `cmd/pcb` and `internal/app`.
- Error handling: handlers return `(string, tea.Cmd, error)`; CLI uses
  `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` + `os.Exit(1)` (`main.go:128`).
- Cobra subcommand nesting: san has none yet → maestro becomes the first nested
  tree (`pcb maestro project add`, `pcb maestro session ls`, etc.). Cobra
  supports this natively; no san framework change needed. **Inferred: cobra
  nesting works without san changes** (unverified — confirm in impl).
- HTMX UI: server renders `text/html` fragments (e.g. `<div id="sessions">…`);
  htmx attributes (`hx-get="/sessions" hx-target="#sessions" hx-swap="outerHTML"`)
  drive partial swaps. No JS SPA. Endpoint can return JSON to CLI & HTML
  fragment to UI based on `Accept` header — one handler, two renders.
- HTTP server reuse: `pcb maestro ui` mirrors `pcb inspector` shape exactly —
  `net.Listen` loopback → `http.Server` → embedded `embed.FS` for static assets
  → `openBrowser`. The only difference: routes proxy to the Maestro daemon.

### Edge cases — inferred
- Daemon not running → HTTP calls fail with connection refused. CLI must give a
  clear "daemon is not running — run `pcb maestro start` or open the desktop app"
  message (maestro's own rule: CLI must not open SQLite directly).
- `running.json` handshake (PID/port) — san should read it to confirm liveness
  before product commands, else a stale PID → confusing errors.
- Spawn resolves project via `--project`/`MAESTRO_PROJECT_ID`/
  `MAESTRO_SESSION_ID`/cwd match → san must forward cwd & respect env.
- `maestro preview` reads session from `MAESTRO_SESSION_ID`, not a flag.
- Windows: conpty built in; no tmux. `proc.DetachSession` has unix+windows files.
- Timeouts: respect `MAESTRO_REQUEST_TIMEOUT` (60s default) on san's http.Client.
- Background spawn (long-lived agent session) → don't block san's CLI; return
  the session id. For TUI, surface the spawned session in a tracker-style list.

### Tests — inferred
- No existing maestro tests (new pkg). Add `internal/maestro/client_test.go`
  using `httptest` to stub daemon routes (mirror `internal/search/*_test.go`).
- CLI smoke: mirror maestro's own manual smoke (`start/status/stop`) but
  gate behind integration tag since it needs a real daemon.
- TUI: slash command handlers are thin; test the client, not the glue.

## Impact radius
- **New files**: `internal/maestro/{client,types,commands}.go` + tests;
  `internal/maestro/ui/{server,embed}.go` + `assets/{index.html,style.css,htmx.min.js}`;
  `cmd/pcb/maestro.go`; `internal/app/input/slash_command_maestro.go` (handler);
  `docs/packages/2-feature/maestro.md`.
- **Modified**: `cmd/pcb/main.go` (register `maestroCmd`), `internal/app/input/slash_command.go`
  (add handler to map), `internal/setting/settings.go` (optional `Maestro` config block:
  port/binary path/timeout/ui port), `docs/packages/index.md` (list new feature).
- **Downstream**: `internal/maestro` is a leaf pkg (like `internal/search`);
  `internal/maestro/ui` imports `internal/maestro`. Only `cmd/pcb` and
  `internal/app` import them. No change to tool registry, agent, session, or
  core contracts. `openBrowser`/`requireLoopback` reused from `cmd/pcb/inspector.go`
  (move to shared util if circular-import risk — see risks).
- **Docs**: new feature doc + update index.

## Steps

1. **Create HTTP client package** (`internal/maestro/client.go`, `types.go`).
   `Client` struct with base URL (default `http://127.0.0.1:3001`, override via
   `MAESTRO_PORT`), `*http.Client` with `MAESTRO_REQUEST_TIMEOUT` (def 60s).
   Methods: `Start/Stop/Status/Doctor` (daemon control), `ProjectAdd/Ls/Get/SetConfig/Rm`,
   `AgentLs(refresh)/Spawn/Send/Preview`, `SessionLs/Get/Kill/Restore/Rename/Cleanup/ClaimPR`,
   `OrchestratorLs`. Each method maps 1:1 to a daemon route. Include `running.json`
   liveness check helper (`IsDaemonRunning`). Mirror `internal/search/exa.go` style.

2. **Add config** (`internal/setting/settings.go:27`): add `Maestro` struct field
   (`Port int`, `BinaryPath string`, `Timeout time.Duration`) with JSON tags, defaults
   (3001, "", 60s). Read via `setting.Default().Data.Maestro.*`. Env vars still override.

3. **CLI subcommand** (`cmd/pcb/maestro.go` + register in `main.go:71-76`):
   `maestroCmd` with nested subcommands mirroring maestro's surface:
   `pcb maestro start|stop|status|doctor` (daemon control),
   `pcb maestro project add|ls|get|set-config|rm`,
   `pcb maestro agent ls [--refresh]`,
   `pcb maestro spawn [--agent --project --skip-agent-check]`,
   `pcb maestro session ls|get|kill|restore|rename|cleanup|claim-pr`,
   `pcb maestro orchestrator ls`,
   `pcb maestro send <session> <msg>`,
   `pcb maestro preview [url]` (reads `MAESTRO_SESSION_ID`),
   `pcb maestro ui` (launch Electron desktop — see step 5).
   Each subcommand: `--json` flag where the daemon supports it; print human-readable
   otherwise. `start` & `ui` shell out to the `to` binary detached
   (`proc.DetachSession` + `proc.SetProcessGroup`); product commands use the HTTP client.
   Handle "daemon not running" with a clear stderr hint.

4. **TUI slash command** (`internal/app/input/slash_command.go:107` map + new
   `slash_command_maestro.go`): add `"maestro"` handler. Sub-actions via args,
   e.g. `/maestro` (status), `/maestro sessions`, `/maestro spawn <agent>`,
   `/maestro send <session> <msg>`, `/maestro ui`. Handler constructs the
   `maestro.Client`, calls the matching method, returns a `tea.Cmd` that emits
   a renderable message (list of sessions / status line). `/maestro ui` boots
   the HTMX server in the background (from step 5) and opens the browser —
   runs the server in a goroutine; the TUI stays interactive.

5. **HTMX web UI server** (`internal/maestro/ui/`): `server.go` with
   `*http.ServeMux`, `New(client *maestro.Client) *Server`, `Handler() http.Handler`.
   Routes (return HTML fragments): `GET /` (dashboard shell), `GET /sessions`
   (list), `GET /sessions/{id}` (detail), `POST /sessions` (spawn form), 
   `POST /sessions/{id}/send`, `POST /sessions/{id}/kill`, `GET /projects`,
   `POST /projects`, `GET /agents`, `GET /status`. Each route calls the
   `maestro.Client`, renders an `html/template` fragment, returns
   `text/html` (htmx swaps via `hx-target`/`hx-swap`). `embed.go` embeds
   `assets/{index.html,style.css,htmx.min.js}` (vendored htmx, ~14KB). Mirror
   `internal/inspector/ui/embed.go` + `internal/inspector/server.go` exactly.
   `cmd/pcb/maestro.go` `ui` subcommand: `net.Listen("tcp","127.0.0.1:0")` →
   `http.Server{Handler: maestroui.New(client).Handler()}` → `openBrowser(url)`
   (reuse `cmd/pcb/inspector.go:118`) → `srv.Serve(ln)`. SIGINT shutdown like
   `inspector.go:73-80`. `requireLoopback` guard (`inspector.go:105`).

6. **Docs** (`docs/packages/2-feature/maestro.md`): purpose, entrypoints
   (`pcb maestro *`, `/maestro`, `pcb maestro ui` web UI), core packages
   (`internal/maestro` + `internal/maestro/ui`), flow (CLI→HTTP→daemon;
   TUI→HTTP→daemon; HTMX UI→loopback server→HTTP→daemon), configuration
   (env vars + settings block), tests, pitfalls (daemon-must-be-running,
   running.json, session env resolution, Windows conpty, loopback-only guard).
   Update `docs/packages/index.md`.

7. **Tests** (`internal/maestro/client_test.go` + `internal/maestro/ui/server_test.go`):
   `httptest.NewServer` stubs for daemon routes, assert request path/method/body
   + response decode. UI tests assert HTML fragment rendering via `httptest`
   + the proxy mapping to the daemon stub. Add an integration-tagged CLI smoke
   test (`start/status/stop/ui`) that skips unless `MAESTRO_INTEGRATION=1`.

## Risks & assumptions
- **Cobra nesting unverified**: san has no nested subcommand tree today; assume
  cobra supports it natively (it does), but confirm no san `rootCmd` quirk
  blocks it during impl. If blocked, flatten to `pcb maestro-*` prefix.
- **Daemon-must-be-running**: san doesn't manage the daemon lifecycle beyond
  `start`/`stop`. Assumes the user runs `to start` or opens the desktop app
  first. If the daemon is down, every product command fails — the plan
  front-loads a clear error + hint, not auto-start (auto-start could surprise
  the user by spawning a long-lived process).
- **`to` vs `maestro` binary name**: README uses `to` (npm pkg `@tinhtran24/to`,
  binary `to`) but the repo/cli docs say `maestro`. Plan assumes `to` is the
  binary on PATH per user confirmation; config field `BinaryPath` lets the user
  override. Verify the exact binary name at impl. Only needed for `start`/`stop`
  daemon control (shelling out); product commands + HTMX UI are pure HTTP.
- **HTMX vendoring**: htmx.min.js (~14KB) must be vendored into
  `internal/maestro/ui/assets/` (offline-friendly, no CDN). License = BSD-2.
  Confirm license compatibility at impl.
- **`openBrowser` location**: currently lives in `cmd/pcb/inspector.go:118`
  (package main). `cmd/pcb/maestro.go` is same package → can call directly. But
  `internal/app` (TUI `/maestro ui`) is a different package and can't import
  `cmd/pcb`. Either (a) move `openBrowser`+`requireLoopback` to a shared util
  pkg (e.g. `internal/webutil`), or (b) have `/maestro ui` shell out to
  `pcb maestro ui` via `proc.DetachSession`. Option (a) is cleaner. Low risk.
- **HTTP surface drift**: maestro is on `dev` branch; routes may change. Pin to
  the routes in `docs/cli/README.md` as of this read; re-check at impl.
- **No SSE/WebSocket**: plan covers REST only (project/agent/session/send).
  Maestro also has SSE events (`/api/v1/events`) & terminal WebSocket — out of
  scope; live-updating HTMX session list could later poll via `hx-trigger="load
  delay:2s"` or use SSE, but not in initial scope.
- **PR/review actions are HTTP-only** (no CLI today) — plan does not expose them;
  can be added later as `pcb maestro pr ...` / `pcb maestro review ...` if needed.

## Unresolved assumptions
- **Cobra nested subcommand tree**: unverified that san's `rootCmd` wiring
  (custom `helpCmd` at `main.go:73-74`, `ArbitraryArgs` root) cleanly accepts a
  nested sub-subcommand tree. Low risk; confirm at first impl step.
- **`openBrowser`/`requireLoopback` sharing**: these live in `cmd/pcb`
  (package main). `internal/app` (TUI) can't import them. Need to move to a
  shared `internal/webutil` pkg, or have the TUI `/maestro ui` shell out to
  `pcb maestro ui` detached. Low risk; decide at impl.
- **Binary name**: `to` vs `maestro` — confirm which is on PATH. Only affects
  `start`/`stop` daemon control (shell-out); product commands + HTMX UI are
  pure HTTP and unaffected.
