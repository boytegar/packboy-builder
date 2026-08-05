# Plan: Image Paste Display + Vision Model Pre-Analysis

## Goal
Two related features for the chat composer: (1) When the user pastes an image with Ctrl+V (already working), show the image's filename — truncated to 5 chars + `-` + extension (e.g. `inigambarsaya.png` → `iniga-.png`) — both as a badge above-left of the textarea and as the inline token inside the textarea (replacing `[Image #N]`). (2) Add a per-role "vision model" setting (new `/models` "Vision" tab, 2-phase pick like the Subagents tab) so that when a user sends a chat with images, a designated vision model first analyzes the image and returns text analysis; the main agent then receives only that analysis merged into the user's prompt (images stripped, so the main model may be text-only).

## Knowledge summary

### Scope — verified
- Chat input is `internal/app/input` wrapping `charm.land/bubbles/v2/textarea` — `internal/app/input/model.go:33-40` (`Textarea`, `Images ImageState`).
- App holds it as `userInput input.Model` — `internal/app/model.go:42`.
- View rendering of the composer: `internal/app/view.go:163-177` (`renderFooter` → writes separator, computes `inputRow` at :176, writes input view :177). A dedicated image/warning line is already budgeted in `fixedChromeLines = 6` — `internal/app/input/on_textarea.go:22`.
- Existing image label fn `imageLabel(id) => "[Image #N]"` — `internal/app/input/on_textarea.go:65`; `AddPendingImage` :70 inserts the token into the textarea.
- Inline image token regex `core.InlineImageTokenRe` — `internal/core/message.go:464`; extractor `ExtractInlineImages` — `internal/app/input/on_textarea.go:197`.
- Submit path: `handleSubmit` `internal/app/update_submit.go:23` → `dispatchSubmission` :68 → `buildUserMessage` :136 → `SubmitToAgent` :206.
- Image gate (blocks turn if main model text-only): `imagesBlockedForModel` `internal/app/update_submit.go:159`; `dropImagesTextOnlyModelRejects` `internal/app/agent.go:492`.
- Settings struct + merge/clone/load plumbing: `internal/setting/settings.go:27-120` (struct), `:830-861` (`Clone`), `internal/setting/merger.go:15-35` (`mergeSettings`), `internal/setting/loader.go:269-289` + an `UpdateXxx` wrapper like `:326-347`.
- `/models` tab enum: `internal/app/input/on_provider.go:28-32`. Subagents tab 2-phase pick UI: `internal/app/input/on_provider_subagents.go:29-42,150-168,257-297`; tab header view `internal/app/input/on_provider_view.go:146-158`. Persisted ref format `"vendor/model"` via `llm.ParseVendorModel` `internal/llm/registry.go:239-245`.

### Entry points — verified
- Ctrl+V/Ctrl+Y paste already routed: `internal/app/update_keys.go:111-112` → `pasteImageFromClipboard` `internal/app/update_input_effects.go:61` → reads clipboard → `AddPendingImage` → inserts label token at :70-72.
- Clipboard filename is synthetic `clipboard_HHMMSS.png` — `internal/image/clipboard.go:24` (real source filename only exists for `@path`/drag-drop via `image.Load` → `filepath.Base`).
- Submit entry: `handleSubmit` Enter key — `internal/app/update_keys.go:178-179`.
- Vision pre-pass insertion point: between `buildUserMessage` (`update_submit.go:136`) and `SubmitToAgent` (`:206`) — a `tea.Cmd` boundary, async LLM call fits Bubble Tea. Closest precedent: async one-shot side-model `missionRefine` `internal/app/autopilot.go:234-253`.

### Dependencies — verified
- One-shot side-model call: `llm.Complete` `internal/llm/types.go:303`; `Client.Complete` `internal/llm/llm.go:164-198`; `resolveReviewerModel` `internal/app/agent.go:307-317` is the closest "designated side model, one call, return text" precedent.
- Cross-vendor connector: `llm.ProviderPool.Resolve` `internal/llm/provider_pool.go:31-53`.
- Merge analysis text into prompt: `reminder.AttachToContent` `internal/reminder/reminder.go:311-326`, `Reminder.Enqueue` via `attachPendingReminders` `internal/app/agent.go:509`.
- Messages already multimodal: `core.Message{Images []Image}` `internal/core/message.go:52-78`; `core.Image{MediaType,Data(base64),FileName,Size}` `:197-203`.
- `core.UserMessage(text, images)` `message.go:237`. Agent ingest: `internal/core/agent_impl.go:305-316` → `streamInfer` `:746` → `a.llm.Infer` `internal/llm/llm.go:76` → `toProviderMessages` `:377-406` (keeps `Images` on user msgs).
- Vision-capability gate: `llm.SupportsImages` `internal/llm/types.go:108-123` (default true; DeepSeek opts out `internal/llm/deepseek/client.go:110-112`).

### Conventions — verified
- Layering: `internal/app` may import features; features must not import app — `docs/reference/dependency-rules.md:37-50`. A vision-analysis helper belongs in `internal/llm` (or a new feature pkg), wired from `internal/app`.
- Role-model refs live in settings.json as `"vendor/model"` strings, resolved at use time via `ParseVendorModel` + `ProviderPool` — precedent `SubagentModels`/`SubagentDefaultModel` `internal/setting/settings.go:83-109`.
- Per-role model pick UI lives in a `/models` tab; 2-phase pick (list → confirm) persists a ref — `on_provider_subagents.go:257-297`.

### Edge cases — verified
- Clipboard pastes have no source filename → badge for Ctrl+V reads `clipb-.png` unless clipboard filename scheme changes. (For `@path`/drag-drop the real name is available.)
- Multiple pending images possible (`Images.Pending` slice) → badge row and inline tokens must handle N labels.
- Main model text-only + vision model set → must bypass `imagesBlockedForModel` (`update_submit.go:159`) and strip images before `dropImagesTextOnlyModelRejects` (`agent.go:492`).
- `toProviderMessages` (`llm/llm.go:377`) drops `DisplayContent` → interleaved text/image ordering lost on live agent path; a vision preprocessor calling `llm.Complete` directly must pass `Images` on the message and accept images-before-text ordering.
- No vision model set, no vision-capable model, vision call fails, empty/zero-byte image, image > 5 MB cap (`internal/image/image.go:17`).

### Tests — verified
- Existing tests to mirror: `internal/app/input/on_textarea_test.go:339,366,380` (image label/token); view test `internal/app/view_test.go`.
- New tests needed: filename truncation helper (5 chars + `-` + ext), badge render with N images, inline token replacement, vision pre-pass cmd (mock side-model returns text → assert images stripped + analysis attached), settings field clone/merge, `/models` Vision tab pick persistence.

## Impact radius
- `internal/app/input/model.go` — `ImageState`/`PendingImage` may carry a display-name field (or derive from FileName).
- `internal/app/input/on_textarea.go` — `imageLabel` (+ `AddPendingImage` token insertion) changed to truncated filename; badge/inline token render.
- `internal/app/input/view.go` — render filename badge(s) above-left of textarea; ensure `inputRow` accounting stays correct.
- `internal/app/view.go:163-177` — insert badge line between separator and input; `inputRow` computed after badge.
- `internal/app/update_input_effects.go:61` (`pasteImageFromClipboard`) — set/carry filename for badge.
- `internal/image/clipboard.go:24` — optional: richer synthetic name or expose source name if available.
- `internal/app/kit/util.go` — new truncation helper `TruncateFilenameKeepExt(name, 5)` (existing `TruncateText`:40, `TruncateKeepEnd`:68 don't do the 5-char+ext shape).
- `internal/setting/settings.go` (+ `merger.go`, `loader.go`) — new `VisionModel string` field + `UpdateVisionModel` wrapper + clone/merge.
- `internal/app/input/on_provider.go` (+ `on_provider_vision.go` new file, `on_provider_view.go`) — new `/models` "Vision" tab, 2-phase pick, persist ref.
- `internal/app/update_submit.go` — insert vision pre-pass cmd between `buildUserMessage` and `SubmitToAgent`; bypass `imagesBlockedForModel` when vision model set; strip images + attach analysis via reminder before `sendToAgent`.
- `internal/app/agent.go:492` — bypass `dropImagesTextOnlyModelRejects` when vision model handled the images.
- `internal/app/autopilot.go` / `internal/app/agent.go:307` pattern — new `resolveVisionModel` + async one-shot `llm.Complete` call.
- New vision-analysis helper (feature pkg or `internal/llm`) per layering rules.
- Tests: `on_textarea_test.go`, `view_test.go`, new settings/merge tests, new vision pre-pass test.
- Downstream: `internal/app/update_keys.go` (token nav/delete keys `on_image.go:9` must match new token shape — **unverified** whether `[Image #N]` regex is hard-wired there).

## Steps
1. **Filename truncation helper** (`internal/app/kit/util.go`) — add `TruncateFilenameKeepExt(name string, prefixLen int) string` producing `iniga-.png` (first 5 chars + `-` + extension incl. dot). Unit-test it.
2. **Carry filename through pending image state** (`internal/app/input/model.go`, `on_textarea.go`) — ensure `PendingImage` exposes `FileName`; `imageLabel` renders truncated filename instead of `[Image #N]`; update `AddPendingImage` token text accordingly. Verify token regex `core.InlineImageTokenRe` and nav/delete keys (`on_image.go`) still match the new token shape, or adapt the regex.
3. **Render badge above textarea** (`internal/app/input/view.go`, `internal/app/view.go:163-177`) — render one truncated-filename badge per pending image left-aligned on the reserved image/warning line; insert before `inputRow` computation so cursor math stays correct. Test via `view_test.go`.
4. **Clipboard filename** (`internal/image/clipboard.go`) — decide display name for Ctrl+V pastes: keep `clipboard_HHMMSS.png` → `clipb-.png`, or accept a caller-supplied name from `pasteImageFromClipboard` (`update_input_effects.go:61`).
5. **Settings field** (`internal/setting/settings.go`, `merger.go`, `loader.go`) — add `VisionModel string`; add `Clone` entry, merge rule, `UpdateVisionModel` wrapper.
6. **`/models` Vision tab** (`internal/app/input/on_provider.go`, new `on_provider_vision.go`, `on_provider_view.go`) — add a tab value, 2-phase pick (list vision-capable models → confirm), persist `"vendor/model"` ref into `VisionModel` setting, following `on_provider_subagents.go` exactly.
7. **Vision pre-pass helper** (new feature pkg or `internal/llm`) — `AnalyzeImages(ctx, ref string, images []core.Image) (string, error)`: resolve model via `ParseVendorModel` + `ProviderPool.Resolve`, build a `core.Message{Role:User, Images, Content:"Analyze this image..."}`, call `llm.Complete`/`Client.Complete`, return text. Per layering rules this sits outside `internal/app`.
8. **Wire pre-pass into submit** (`internal/app/update_submit.go`) — after `buildUserMessage`, when images present and `VisionModel` set: bypass `imagesBlockedForModel`; emit an async `tea.Cmd` calling the helper; on completion, attach analysis to the prompt via `reminder.AttachToContent`/`Reminder.Enqueue`, strip `Images` from the message, then call `SubmitToAgent`/`sendToAgent`. Model the cmd on `missionRefine` (`autopilot.go:234`).
9. **Bypass text-only strip** (`internal/app/agent.go:492`) — when images were routed through the vision model, skip `dropImagesTextOnlyModelRejects` for the turn.
10. **Failure UX** — vision model unset + main model text-only → keep current `imagesBlockedForModel` block with a hint to set a vision model; vision call error → surface as a transient warning on the reserved line, do not send images to main model.
11. **Tests** — truncation helper unit test; badge render test (`view_test.go`); inline-token replace test (`on_textarea_test.go`); settings clone/merge test; `/models` Vision tab persistence test; vision pre-pass test (mock `llm.Complete` → assert analysis attached + images stripped); bypass of `dropImagesTextOnlyModelRejects` test.

## Risks & assumptions
- **Token shape change breaks nav/delete keys.** `core.InlineImageTokenRe` and `HandleImageSelectKey` (`on_image.go:9`) may be hard-wired to `[Image #N]`. Changing the inline token to a filename must keep a stable, regex-matchable shape (e.g. keep an index `[iniga-.png #2]` or a sentinel wrapper) — needs verification before editing (marked unverified in impact radius).
- **Ordering loss in `toProviderMessages`** (`llm/llm.go:377`) — the vision helper calls `llm.Complete` directly, so it must construct `core.Message` with `Images` set and a text instruction; image-before-text ordering is acceptable for analysis.
- **Clipboard has no real filename** — Ctrl+V badge will show `clipb-.png`; acceptable per existing synthetic scheme but not matching the user's `inigambarsaya.png` example (which only arises from `@path`/drag-drop).
- **Main model text-only assumption inverted** — after vision routing, `imagesBlockedForModel` and `dropImagesTextOnlyModelRejects` both assume the main model handles images; both bypasses must be gated precisely on "vision model set AND images were routed through it" to avoid regressing the text-only guard.
- **Vision model may not support images** — the Vision tab should filter to `llm.SupportsImages` models, but catalogs don't carry vision metadata (`ModelInfo` has no vision flag); filtering may rely on the default-true `SupportsImages` and the DeepSeek opt-out only.
- **Reminder merge semantics** — attaching analysis via `Reminder.Enqueue`/`AttachToContent` lands it as a `<system-reminder>` on the user turn; assumes this is the desired "merge with prompt" shape (cleanest available merge point).

## Unresolved assumptions
None — confidence at 90%+. (One item marked unverified in Impact radius: whether `[Image #N]` is hard-wired in `on_image.go:9` nav/delete keys; to be confirmed at implementation time and is a code-edit detail, not a planning gap.)
