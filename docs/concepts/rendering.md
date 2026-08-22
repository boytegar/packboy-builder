# Rendering: Model → Terminal

> 中文版本：[`rendering.zh.md`](rendering.zh.md)

Companion to [`data-flow.md`](data-flow.md). Data flow covers how input
becomes state; this doc covers how state becomes characters on screen.

> **"Render" means: return a string.** Every `Render*` function in
> this codebase returns a `string` — ANSI escape codes for color and
> style, plain UTF-8 for content. No off-screen buffers, no canvases.
> Bubble Tea's `View()` returns a string and the framework writes it
> to the terminal. Since the full-window (alt-screen) rewrite, all
> conversation content renders inside one in-app viewport
> (`charm.land/bubbles/v2/viewport`) that scrolls in place; the
> terminal's native scrollback is no longer used for chat history.

## Mental model: one surface, one viewport

The terminal runs in the **alternate screen buffer** (`View` sets
`AltScreen = true`). Every message — committed and live — renders into
one in-app viewport that fills the window above the pinned input strip:

```
m.conv.Messages = [ msg0, msg1, msg2, msg3 | msg4, msg5 ]
                                            ▲
                                            CommittedCount = 4

┌─ surface ────────────────┬─ written by ──┬─ contents ────────────────────┐
│ Chat viewport (one pane) │ View()        │ renderedBlocks (msg0..msg3)   │
│  fills the window above  │ + commit args │ + live tail (msg4..msg5,      │
│  the input; ScrollUp/    │               │   redrawn every frame)        │
│  ScrollDown pages it     │               │ + tracker / status overview   │
├──────────────────────────┼───────────────┼───────────────────────────────┤
│ Footer (pinned bottom)   │ View()        │ queue preview, separator,     │
│                          │               │ textarea, mode-status line    │
└──────────────────────────┴───────────────┴───────────────────────────────┘
```

The chat viewport is **always the same height** (terminal height minus
the footer); only its scroll offset changes as the conversation grows.
Scrolling is in-app: wheel / PgUp / PgDn adjust the viewport's Y offset,
and when the user scrolls up past the bottom a "▼ End to return to
latest" band appears above the input. Follow mode (bottom-pinned, the
default) makes the view drift down as streaming blocks commit.

A **streaming** assistant reply renders into the live tail while
`Stream.Active == true`; when a block completes, the off-thread flush
renders it and `handleFlushResult` **appends** it to the committed
render cache (`chatView.renderedBlocks`), advancing the message's commit
offsets so the live tail stops redrawing it. The user sees one visual
transition, not a duplicate. The rule that prevents double-rendering is
in `renderAndCommit(checkReady=true)`: never commit the last message
while `Stream.Active` is true.

**Both committed and live content share the same render functions.**
`RenderMessageAt` is what produces each message's string; what differs
is where the result goes — committed rows join the cache once (never
re-parsed), the live tail re-renders every frame.

## View() composes the alt-screen frame

`(*model).View()` in [`internal/app/view.go`](../../internal/app/view.go)
runs after every `Update` and returns the string for the chat viewport.

```go
func (m *model) View() tea.View {
    //   ^ Go method on *model; `m` is the instance (Go's
    //     equivalent of `this`/`self`). The whole codebase uses `m`
    //     for the foreground model.
    ...
}
```

**No input parameters** — that's a Bubble Tea contract. Everything
View reads comes from `m`'s fields. The sub-renderers it calls
(`RenderActiveContent` etc.) take a `conv.RenderContext` struct, which
`m.messageRenderParams()` assembles from `m`'s fields on each call.
`View` also enables mouse reporting (`MouseModeCellMotion`) and routes
wheel events to the model via the `OnMouse` closure — the closure only
packages a `scrollMsg`; all state mutation happens in `Update`.

View() picks one of four layouts, top-down:

```
View()
  1. !m.env.Ready              ──► "\n  Loading..."
  2. active popup?             ──► popup.Render() — fullscreen
                                   (slash-command pickers: /models,
                                   /tools, /skills, ...)
  3. active modal?             ──► modal.Render() wrapped between
                                   separator bars
                                   (Question modal, Approval modal)
  4. otherwise (normal mode) ──► renderNormalView()
        ├─ chat viewport        ── cached committed blocks + live tail,
        │                          scrolled in place (wheel / PgUp/PgDn)
        ├─ [▼ End to return to latest]  ── only while scrolled up
        ├─ separator
        ├─ queue preview       ── if input was queued during a stream
        ├─ textarea
        ├─ suggestion list     ── /-command and @-file autocomplete
        ├─ separator
        └─ status line         ── model name, tokens, mode
```

Popups (full-screen) and modals (wrapped) look like the same idea but
have different render flows because the chat content stays visible
behind a modal, not behind a popup.

## How a single message is rendered

`RenderMessageAt(ctx, idx, isStreaming)` dispatches by `msg.Role`:

```
                ┌── Role: User ──┐
                │                │
                │ ToolResult?    ──► RenderToolResultInline
                │ otherwise      ──► RenderUserMessage
                │                     (text + images, md-rendered)
                │
RenderMessageAt ─┤
                │
                ├── Role: Notice ──► RenderSystemMessage
                │                     (plain text, muted color)
                │
                └── Role: Assistant ──► renderAssistantWithTools
                                          ├─ assistant text + thinking
                                          │   (md-rendered)
                                          └─ tool-calls block (each
                                              call + its inlined
                                              result, if available)
```

`renderAssistantWithTools` does **not** scan the message list to find
its paired results — `ctx.InlinedResults` was precomputed once at the
top of the render pass and tells it which `ToolCallID → ToolResultData`
entries to inline. See "Tool calls and inlined results" below.

## Markdown via MDRenderer

[`internal/app/conv/markdown.go`](../../internal/app/conv/markdown.go)
wraps [glamour](https://github.com/charmbracelet/glamour). Five
behaviors are intentional and not glamour defaults:

| Concern | Behavior |
| --- | --- |
| Width | Built for the current terminal width, minus 4 for the `● ` indent. `ResizeMDRenderer` rebuilds it on `WindowSizeMsg`. |
| Background | Auto-detects dark vs light. `rebuildIfNeeded()` rebuilds inside `Render` if the terminal flipped themes. |
| Tables | Pulled out before glamour sees them; rendered with lipgloss table primitives for full border control. |
| Soft line breaks | LLMs hard-wrap at ~80 cols. Soft-wrapped paragraphs get joined before glamour so it can re-wrap at the real width. |
| Inline tokens | A custom inline-markdown pass styles things glamour handles poorly (e.g. backticks inside other formatting). |

Width matters: glamour computes column widths from its configured
width. If the terminal resizes, glamour-wrapped blocks already in the
viewport cache are sized for the old width, so they must be re-rendered.
That mismatch is exactly what `reflowCommitted` addresses (see Resize
below).

## Tool calls and inlined results

[`internal/app/conv/tool_render.go`](../../internal/app/conv/tool_render.go)
renders the tool-calls block under an assistant message:

```
● Bash(npm test)                        ← tool name + summary args
    ⎿  > vitest run                     ← collapsed result preview
        ✓ src/foo.test.ts (12)
        ✓ src/bar.test.ts (8)
       … 47 more lines (Ctrl-O to expand)
```

State that drives it:

- **Pending vs done** — a tool call sits in `m.conv.Tool.PendingCalls`
  until its `ToolResult` arrives. While pending, the tool name shows
  a spinner.
- **Expanded / collapsed** — per-message `Expanded` flag, toggled by
  Ctrl-O. Collapsed = preview + line count; expanded = full content.
- **Error** — `ToolResult.IsError` flips the icon ✓ → ✗ and tints the result.
- **Parallel mode** — when multiple tool calls run in parallel, each
  call shows its own progress. `PreTool` is emitted at each call's real
  start (not batch resolve), so elapsed timers stay independent under
  mixed Agent + read-only batches. `ToolExecState.Completed` tracks
  out-of-order finishes so completing the last-indexed call early does
  not clear the batch.

The pairing between an assistant's tool calls and their result messages
is precomputed by `PrecomputeInlinedResults(messages)` and lives on
`RenderContext.InlinedResults`. Three lookups consume it:

```
InlinedResults.ownerOf(resultIdx)      // which assistant owns this result?
                                        // used by RenderMessageRange to skip
                                        // the result (it's drawn inline)

InlinedResults.resultsFor(assistantIdx) // (callID → ToolResultData) for an
                                        // assistant; used by
                                        // renderAssistantWithTools

InlinedResults.IsResultInlined(idx)     // is this result already going to
                                        // be drawn under its owner?
                                        // used by RenderSingleMessage to
                                        // skip standalone Println
```

One pass over the message list, three consumers, zero re-scanning.

## Worked example: streaming reply + tool call

End-to-end trace. The user typed `list files` and pressed Enter; that
part is the input flow ([data-flow.md](data-flow.md) Path A). Below
picks up at the moment the agent goroutine starts emitting events.

`conv.Messages` starts as `[user "list files"]` with
`CommittedCount=1` (the user message was already committed by the
Enter handler).

### Step 1 — PreInfer: open an empty assistant stub

```
event:           core.PreInfer
applyPreInfer:   m.Stream.Active = true
                 m.Append({Role: assistant, Content: ""})
                 start spinner

conv.Messages:   [user, assistant{Content:""}]
CommittedCount:  1   (only user is committed so far)
```

View() runs after this Update:

```
View → renderNormalView
     → conv.RenderActiveContent(ctx)
       ctx.InlinedResults = PrecomputeInlinedResults(Messages)
         = {} (no ToolCalls anywhere yet)
       → RenderMessageRange(ctx, startIdx=1, endIdx=2, includeSpinner=true)
         i=1: ownerOf(1) = -1 (not a result) → don't skip
              isStreaming = (1 == lastIdx && Stream.Active && role==assistant)
                          = true
              → RenderMessageAt(ctx, 1, isStreaming=true)
                → renderAssistantWithTools(ctx, msg, 1, isLast=true)
                  → RenderAssistantMessage(content="", streamActive=true,...)
                    returns the "● ▮" stub
                  msg.ToolCalls == nil → just return base
         + pending-tool spinner
```

Repaint zone shows `● ▮ ⋯`. Scrollback unchanged.

### Step 2 — OnChunk (text): grow the message

```
event:           core.OnChunk{Text: "I'll list them with ls.", Done: false}
applyChunk:      m.AppendToLast(text, "")

conv.Messages:   [user, assistant{Content:"I'll list them with ls."}]
Stream.Active:   still true (Done=false)
```

Same call chain as Step 1, but `RenderAssistantMessage` now has
non-empty content and `MDRenderer.Render` styles it. Repaint zone:
`● I'll list them with ls. ▮ ⋯`. More OnChunks may follow — each is
`AppendToLast` + a View() repaint.

### Step 3 — PostInfer: tool calls land on the assistant message

```
event:           core.PostInfer{Response: {ToolCalls: [{ID:"tc-1", Name:"Bash", Input:{cmd:"ls"}}]}}
applyPostInfer:  rt.OnTokenUsage(resp)
                 m.SetLastToolCalls(resp.ToolCalls)
                 m.Tool.Track(resp.ToolCalls)

conv.Messages:   [user,
                  assistant{Content:"I'll list them with ls.", ToolCalls:[tc-1]}]
```

`renderAssistantWithTools` now takes the second branch:

```
base = RenderAssistantMessage(...)             ← the text part
msg.ToolCalls != nil
resultMap = ctx.InlinedResults.resultsFor(1)
          = nil                                 ← tc-1 hasn't finished
RenderToolCalls(ToolCallsParams{
  ToolCalls:    [tc-1],
  ResultMap:    {},                             ← nil → empty
  PendingCalls: [tc-1],                         ← spinner driver
  CurrentIdx:   0,
  SpinnerView:  "⋯",
  ...
})
```

Repaint zone now shows:

```
● I'll list them with ls.
  ⋯ Bash(ls)
```

### Step 4 — PostTool: result arrives, gets inlined

```
event:           core.PostTool{Result: {ToolCallID:"tc-1", Content:"file1\nfile2"}}
m.ProcessToolResult(tr):
  applyToolSideEffects(...)
  firePostToolHook(...)
  (the agent appends the ToolResult as a user-role message)

conv.Messages:   [user "list files",
                  assistant{Content+ToolCalls:[tc-1]},
                  user{ToolResult:{ToolCallID:"tc-1", Content:"file1\nfile2"}}]
```

View() rebuilds `ctx`. **InlinedResults earns its keep:**

```
PrecomputeInlinedResults(Messages):
  i=1 is an assistant with ToolCalls [tc-1]; scan forward:
    j=2: ToolResult.ToolCallID == "tc-1" → pair
  resultOwner         = {2: 1}
  resultsForAssistant = {1: {"tc-1": ToolResultData{Content:"file1\nfile2", ...}}}

RenderMessageRange(ctx, 1, 3, includeSpinner=true):
  i=1 (assistant):
    ownerOf(1) = -1 (not a result) → render
    renderAssistantWithTools:
      resultMap = resultsFor(1) = {"tc-1": ToolResultData{...}}    ← populated now
      RenderToolCalls draws "● Bash(ls)" with the file listing INLINE below
  i=2 (ToolResult):
    ownerOf(2) = 1, which is >= startIdx → SKIP
    (already drawn under its owning assistant; standalone render would duplicate)
```

Repaint zone:

```
● I'll list them with ls.
  ● Bash(ls)
      ⎿  file1
         file2
```

### Step 5 — OnChunk(Done): promote the block to the viewport cache

```
event:           core.OnChunk{Done: true, Response: {...}}
applyChunk:      m.AppendToLast(...)       (possibly a final text chunk)
                 if chunk.Done && no tool calls remaining:
                     m.Stream.Active = false
                     return rt.CommitMessages()
```

`CommitMessages → renderAndCommit(checkReady=true)`:

```
for i in CommittedCount..len(Messages):    // i = 1, 2
  msg = Messages[i]
  if checkReady && i == lastIdx && role==assistant && Stream.Active:
      break                                  // but Stream.Active is now false
  rendered = conv.RenderSingleMessage(ctx, i)
    i=1: RenderMessageAt(ctx, 1, false)      // no longer streaming → no cursor
         returns the same assistant + tool block as before
    i=2: msg.ToolResult != nil
         InlinedResults.IsResultInlined(2) = true → return ""       ← skipped
  if rendered != "": append to parts

m.chat.appendBlock(strings.Join(parts, "\n")) // ONE append to the render cache
CommittedCount = 3                           // caught up
```

What changed on screen:

- **Viewport cache** gains one block:
  `● I'll list them with ls. / ● Bash(ls) / ⎿ file1 / file2`.
- The live tail is now empty (`CommittedCount == len(Messages)`); the
  next `View()` re-slices the viewport over the grown cache.
- Follow mode (default) keeps the view pinned to the bottom, so the new
  block appears to scroll in as it commits.

The user watched the same string grow in the live tail; now that same
string lives in the committed cache, written exactly once. The
`IsResultInlined` short-circuit in `RenderSingleMessage` is what stops
the ToolResult from also being appended standalone.

### Streaming mid-flight flush

While a stream is still active, completed markdown blocks leave the
live tail early via the flush pipeline (`FlushStreamingBlocks` →
`renderSnapshotCmd` off-thread → `handleFlushResult`):

```
handleFlushResult:  row.ThinkingCommittedLen / ContentCommittedLen advance
                    m.chat.appendBlock(rendered)
                    (nothing is printed to the terminal)
```

This is what makes long assistant replies scroll through the viewport in
paragraph-sized steps instead of jumping all at once at turn end — same
mechanism, no `tea.Println`.

## Resize behavior

Terminal resize is the **only event that invalidates already-rendered
viewport blocks** (glamour wraps at its configured width).
`handleWindowResize` in
[`internal/app/update_resize.go`](../../internal/app/update_resize.go):

1. Update `m.env.Width / Height` and the textarea width.
2. `m.conv.ResizeMDRenderer(newWidth)` — rebuilds glamour at the new
   width.
3. If width actually changed and any messages are committed:
   `reflowCommitted` re-renders every committed message at the new width
   and `chatView.rebuildCache` replaces the cache in one go.
4. Bubble Tea calls `View()` next; `syncSizeIfNeeded` re-slices the
   viewport at the new size and, in follow mode, re-pins to bottom.

Resize is the one path that re-renders the whole history. All other
frames are incremental: appends only.

## File pointers

| Concern | File |
| --- | --- |
| `View()` composition | [`internal/app/view.go`](../../internal/app/view.go) |
| Chat viewport state (cache, follow, scroll) | [`internal/app/model_viewport.go`](../../internal/app/model_viewport.go) |
| Per-message rendering + pairing | [`internal/app/conv/view.go`](../../internal/app/conv/view.go) |
| User / assistant / notice rendering | [`internal/app/conv/message.go`](../../internal/app/conv/message.go) |
| Markdown rendering | [`internal/app/conv/markdown.go`](../../internal/app/conv/markdown.go) |
| Tool call / result rendering | [`internal/app/conv/tool_render.go`](../../internal/app/conv/tool_render.go) |
| Compact / progress / tracker | [`internal/app/conv/compact.go`](../../internal/app/conv/compact.go), [`progress.go`](../../internal/app/conv/progress.go), [`tracker_view.go`](../../internal/app/conv/tracker_view.go) |
| `MDRenderer` lifecycle | [`internal/app/conv/model.go`](../../internal/app/conv/model.go) |
| Commit pipeline (cache append) | [`internal/app/model_scrollback.go`](../../internal/app/model_scrollback.go) |
| Resize + reflow | [`internal/app/update_resize.go`](../../internal/app/update_resize.go) |
