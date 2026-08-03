# Plan: Swarm Delegation Discipline

## Goal
Satu perubahan di ModeSwarm:
1. **Main agent wajib delegasikan eksplorasi (read >1-2 file, telusuri call path, /plan, /research) ke subagent** — main hanya menerima summary. Diterapkan *soft* lewat `swarmPersonaOverlay` (prompt), bukan gate permission hard.

## Knowledge summary

### Scope & entry points
- `OperationMode` enum ada: `ModeNormal / ModeReadOnly / ModeSwarm` (`internal/setting/settings.go:666-676`). `ModeSwarm` = "agent: every turn decomposes and runs parallel subagents".
- Toggle cycle `ModeNormal → ModeReadOnly → ModeSwarm → ModeNormal` di `cycleModes` (`settings.go:685`). Mode live di `env.OperationMode` (`internal/app/env.go:52`, default `ModeNormal` di `:83`).
- `SessionMode()` stringifier: `ModeSwarm` → `"agent"` (`env.go:249-262`).
- System prompt main: `personaPrompt()` (`internal/app/agent.go:59-80`). Saat `ModeSwarm` → layer `swarmPersonaOverlay()` (`agent.go:86-91`) ke Behavior + Rules persona. **Ini injection point untuk aturan delegasi.**
- BuildParams lengkap: `agent.go:163-300` (Persona `:174`, AgentDirectory `:173`, disabled tools `:176`, MCP `:177`, hooks `:178`).
- Subagent executor wiring: `agent.go:700-728` (`subagent.NewExecutor`, `SetExecutor` ke `tool.ToolAgent` + `tool.ToolSendMessage`).

### Subagent dispatch & result flow
- Agent tool interface: `tool.AgentExecutor` (`internal/tool/agent.go:49-57`) — `Run(ctx, req)` sync, `RunBackground(req)` async.
- Sync: `internal/subagent/executor.go:204-229`. Async: `:232-297` — validasi `:299-309` (mode hanya `""|"default"|"explore"|"edit"`), bikin `task.AgentTask` (`:253`), goroutine (`:262`), output → `agentTask.AppendOutput`, `Complete(nil/err)`.
- Parallel results drain: `conv.DrainAgentOutbox(m.services.Agent.Outbox())` (`agent.go:457`).
- **Tidak ada fan-out eksplisit di code** — swarm mode prompt-driven: Behavior text di `agent.go:88` instruksikan model issue multiple Agent tool call dalam satu message.

### Permission/mode plumbing (subagent)
- `validateRequest` (`executor.go:299-309`): mode valid = `""|"default"|"explore"|"edit"`.
- `requestPermissionMode` (`executor_prompt.go:167-184`): `explore` → `PermissionExplore`; `edit` → `PermissionAcceptEdits`; `""|"default"` + agent dikenal → `config.PermissionMode` (frontmatter `mode`); unnamed/displayOnly → snapshot parent.
- `PermissionMode` constants (`internal/subagent/types.go:17-40`): `PermissionDefault / PermissionAcceptEdits / PermissionExplore / PermissionBypass / PermissionDontAsk`. `PermissionExplore` = "reads auto, mutations explicitly Deny".
- `filterSchemasForPermission` (`executor.go:802-819`): whitelist `AllowTools` mempersempit; else `modeAllowsSchema(mode, name)` (`:821-842`). `modeAllowsSchema`: safe tools + SendMessage always; `PermissionAcceptEdits` tambah `perm.IsEditTool`; bypass/auto all; Bash+Skill always visible.
- `subagentPermissionFunc` (`executor.go:710-...`): deny_tools → circuit breaker → bypass-all → confirmation-tier deny → safe-tool auto-permit → allow_tools whitelist → fallback deny. Explore = authoritative read-only boundary, allow_tools tidak bisa elevate ke mutasi.

### Agent config / frontmatter
- `AgentConfig` struct (`internal/subagent/types.go:301-338`): field `Name, Description, Model, PermissionMode (yaml:"mode"), AllowTools, DenyTools, Skills, SystemPrompt, MaxSteps, Source, McpServers, SourceFile`.
- Frontmatter parse: `parseAgentFile` (`loader.go:177-226`), alias `frontmatterAliases` (`loader.go:154-175`) dukung `tools`/`allowed-tools`/`permission-mode`/`max_steps`. Default `Model="inherit"`, `MaxSteps=defaultMaxSteps`, `PermissionMode=PermissionDefault`.
- `frontmatterAliases.applyTo` (`loader.go:161-175`): apply kalau canonical kosong. **Tidak ada alias untuk field write/edit baru** — perlu tambah.

### Konvensi yang dipakai
- Mode permission lewat enum string + `NormalizePermissionMode`. Edit/acceptEdits sudah ada mapping (`types.go:49-57`: `"edit","acceptedits"` → `PermissionAcceptEdits`).
- Per-agent allow/deny via frontmatter YAML, canonical `yaml:"allow_tools"`/`yaml:"mode"`.
- Injection prompt-mode lewat `swarmPersonaOverlay` (sudah pattern established).
- Tool-name constants di `internal/tool/schema.go` (`ToolRead, ToolEdit, ToolWrite, ToolBash, ...`).

## Impact radius
File berubah:
- `internal/app/agent.go` — ekspansi `swarmPersonaOverlay()` (`:86-91`) tambah aturan delegasi eksplorasi + auto-create agent di global folder.
- `internal/subagent/types.go` — tambah field `AllowWrite bool` di `AgentConfig` (`:301-338`) + alias frontmatter.
- `internal/subagent/loader.go` — `frontmatterAliases` (`:154-159`) + `applyTo` (`:161-175`) dukung `allow_write`/`allow-write`. Sediakan `ReloadAgents(cwd)` (atau pakai `LoadAgents` publik) untuk auto-reload.
- `internal/subagent/executor_prompt.go:167-184` — `requestPermissionMode`: `explore` → `PermissionExplore`; `edit` → `PermissionAcceptEdits`; `""|"default"` + agent dikenal → `config.PermissionMode` (frontmatter `mode`); unnamed/displayOnly → snapshot parent. Tidak berubah.

Downstream:
- Agent definition markdown custom (`~/.packboy/agents/*.md`, `<repo>/.packboy/agents/*.md`) bisa set `allow_write: true`.
- Registry reload otomatis setelah Write ke `~/.pcb/agents/` → turn berikutnya agent langsung tersedia.
- Test: `internal/subagent/executor_test.go` (sudah ada test `requestPermissionMode` `:938-978`) — tambah case `AllowWrite=true` → `PermissionAcceptEdits`.
- Test frontmatter parse (`loader_test.go` bila ada) — tambah case `allow_write: true` parse.
- Test loader priority (`loader_priority_test.go`) — tambah case: Write file baru → `LoadAgents` ulang → `LookupAgent` temukan.

## Steps

### A. Delegasi eksplorasi di ModeSwarm (soft / prompt)
1. `internal/app/agent.go:86-91` — ekspansi `swarmPersonaOverlay()`:
   - Behavior: tambah "For any investigation that touches more than 1-2 files, traces call paths, or answers architecture/where-does-X-live questions, you MUST delegate to a `researcher` subagent (mode=explore) instead of reading/grepping yourself. You only receive the subagent's summary. The exception is a single direct Read/Grep when the exact file and target are already known."
   - Rules: tambah "Never use your own Read/Grep/Glob for multi-file exploration in ModeSwarm — dispatch a researcher. Each delegation prompt must be self-contained with all context."
2. Verifikasi prompt builder (`personaPrompt` `:59-80`) sudah append overlay dengan benar ke Behavior+Rules (sudah — tidak butuh perubahan mekanisme).

### B. Per-agent allow_write opt-in
3. `internal/subagent/types.go` — tambah field di `AgentConfig` (`:301-338`):
   ```go
   // AllowWrite, saat true & tidak ada req.Mode eksplisit, default subagent ke
   // PermissionAcceptEdits (write/edit diizinkan tanpa konfirmasi tiap call).
   AllowWrite bool `yaml:"allow_write,omitempty" json:"allow_write,omitempty"`
   ```
4. `internal/subagent/loader.go:154-175` — tambah alias spelling di `frontmatterAliases`:
   ```go
   AllowWrite bool `yaml:"allow-write"`
   ```
   dan di `applyTo`:
   ```go
   if !config.AllowWrite && a.AllowWrite { config.AllowWrite = a.AllowWrite }
   ```
5. `internal/subagent/executor_prompt.go:167-184` — modifikasi `requestPermissionMode`:
   - Di case `case "", "default":` (saat ini return `config.PermissionMode`), tambah: jika `config.AllowWrite && config.PermissionMode == ""` (atau `== PermissionDefault`) → return `PermissionAcceptEdits`. Pertahankan: `req.Mode` eksplisit (`explore`/`edit`) tetap menang.
6. `internal/subagent/executor_test.go` — tambah test case:
   - `AllowWrite=true`, `req.Mode=""` → expect `PermissionAcceptEdits`.
   - `AllowWrite=true`, `req.Mode="explore"` → expect `PermissionExplore` (req wins).
   - Frontmatter parse `allow_write: true` → `config.AllowWrite == true`.

### C. Dokumentasi (opsional, ikut convention AGENTS.md "update docs in same change")
7. `docs/guides/writing-a-subagent.md` — section frontmatter: tambah `allow_write` (bool, default false). Jelaskan: mengaktifkan write/edit tanpa runtime override.
8. `internal/app/input/on_agent.go` — (opsional) UI toggle di agent management screen.

### C. Main dapat menambahkan agent di global folder jika tidak ada
9. `internal/app/agent.go:86-91` — ekspansi `swarmPersonaOverlay()` (gabung dengan section A):
   - Behavior tambah: "If no existing agent definition fits a subtask's need, first create one in the global user folder: write `~/.pcb/agents/<name>/AGENT.md` via the Write tool with YAML frontmatter (`name`, `description`, `model: inherit`, `max_steps`, `allow_write:` bila write needed) and a purposeful system prompt body. Then dispatch the new agent via the Agent tool in the same turn. Prefer the global `~/.pcb/agents/` over project `.pcb/agents/` for reusable agents."
   - Rules tambah: "When creating a new agent on the fly, keep the system prompt self-contained and specific to the subtask. After writing the file, reload is automatic — do not ask the user to restart."
10. Auto-reload registry setelah file write:
    - `internal/subagent/loader.go:62` `LoadAgents(cwd)` sudah re-scan semua path (`:68-73`) termasuk `~/.pcb/agents/` (`:70`). Tapi `LoadAgents` saat ini hanya dipanggil startup (`internal/subagent/service.go:35`).
    - Tambah mekanisme reload: `internal/subagent/loader.go` — buat `ReloadAgents(cwd string)` yang wraps `ClearPluginAgentPaths` no-op + `LoadAgents` (re-register ke `defaultRegistry`; `Registry.Register` overwrite by name → aman, builtin + existing tetap). Atau expose `LoadAgents` lagi (sudah publik) — tinggal panggil ulang.
    - Wiring: `internal/app/agent.go` `BuildParams` (`:163-300`) atau `AgentDirectory` getter (`:173`) harus refresh. Cek `AgentDirectory` getter — bila func() lazily read registry, reload otomatis terlihat di prompt berikutnya. Bila cached → perlu invalidasi.
    - **Minimum viable**: pastikan `AgentDirectory` (`agent.go:173`) adalah getter func() yang baca `subagent.Default()` live (bukan snapshot). Kalau sudah live → setelah `LoadAgents` re-call, turn berikutnya prompt otomatis lihat agent baru. Tidak perlu hook khusus di Write tool.
    - Trigger `LoadAgents` ulang: opsi (a) main agent eksplisit call `/reload-agents` setelah Write — tidak otomatis; opsi (b) hook setelah Write tool ke path `~/.pcb/agents/**` → panggil `subagent.LoadAgents(cwd)`. **Pilih (b)** — auto, transparent. Implementasi: di `internal/tool/write` executor atau `internal/hook` post-write, match path prefix `confdir.Dir(homeDir)+"/agents/"` → `subagent.LoadAgents(cwd)`.
11. Verifikasi `Registry.Register` (`internal/subagent/registry.go:29`) overwrite-by-name → agent baru dengan nama sama dgn builtin akan override (sudah by-design, `LoadAgents` `:64` register builtin first). Aman.
12. Path konvensi: `~/.pcb/agents/<name>/AGENT.md` (subfolder + `AGENT.md`, sesuai `loadAgentsFromDirWithNamespace` `loader.go:106-113`). Pastikan prompt overlay instruksi format ini (bukan flat `<name>.md`).
13. (Opsional) Slash command `/add agent <name> <desc>` saat ini (`slash_command.go:700`) hanya write stub ke project `.pcb/agents/`. Bisa extend: tambah flag `--global` → write ke `~/.pcb/agents/`. Tapi utama = prompt overlay (step 9), bukan slash.

## Risks & assumptions
- **Soft enforcement = LLM mungkin masih Read/Grep sendiri**. Karena pilihan user = "prompt overlay saja", ini diterima. Bila makin ketat nanti, perlu hard gate di `PermissionRules` (`agent.go:201-227`) — deny Read/Grep saat `ModeSwarm` (risk false-positive kerja legit 1-file).
- `AllowWrite=true` + `PermissionMode: "explore"` di frontmatter yang sama = ambigu. Asumsi: `PermissionMode` eksplisit menang (explore lebih ketat). `requestPermissionMode` di step 5 harus eksplisit: `config.PermissionMode != "" && != PermissionDefault` → pakai `config.PermissionMode`, abaikan `AllowWrite`. Hanya saat `PermissionMode==""||"default"` & `AllowWrite` → `PermissionAcceptEdits`.
- `PermissionAcceptEdits` masih prompt tiap mutasi? Tidak — lihat `subagentPermissionFunc`: acceptEdits = auto-permit edit tools (no Ask). Benar untuk subagent (Ask→Deny). Jadi `AllowWrite=true` = write tanpa konfirmasi — sesuai keinginan "support write/edit".
- Parent-only tools (Agent/AgentStop/SendMessage manipulasi state) tetap tidak bisa diakses subagent walau `AllowWrite` (`subagentPermissionFunc` parent-only check di `:737`). Aman.
- `mode: edit` runtime override sudah ada (`requestPermissionMode` case "edit"). `AllowWrite` tidak menghapus itu — hanya default saat mode kosong.
- Circuit breaker (rm -rf root/home) tetap hard-deny di semua mode. Aman.

- **Auto-reload path-prefix match**: hook post-Write ke `~/.pcb/agents/**` bisa false-positive kalau main nulis file lain di folder itu. Asumsi: folder `agents/` khusus AGENT.md → aman. Bila ragu, match suffix `/AGENT.md` saja.
- `AgentDirectory` getter (`agent.go:173`) — belum diverifikasi apakah func() live atau snapshot. **Unverified** — perlu cek sebelum implementasi step 10. Bila snapshot → perlu invalidasi eksplisit setelah reload.
