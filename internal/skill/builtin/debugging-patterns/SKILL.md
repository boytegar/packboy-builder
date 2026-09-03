---
name: debugging-patterns
description: "Systematic debugging methodology for Go applications: reproduction, isolation, root cause analysis, and fix verification. Use when diagnosing bugs, crashes, or unexpected behavior."
origin: builtin
---


Systematic debugging for Go applications. Follow the methodology end-to-end —
skipping steps produces fixes that mask symptoms and recur under load.


# When to use

- A panic, crash, or hang is reported in production, tests, or local runs.
- Behavior diverges from expectations: wrong output, missing data, deadlock.
- A goroutine leak, memory growth, or CPU spike is suspected.
- `go test -race` reports a data race.
- A failing test cannot be explained by reading the code.

This skill assumes a Go monorepo or single-module project using the standard
`go` toolchain. Adapt tool commands to your build system, but do not skip the
methodology.


# Core methodology

Debugging is not guessing. It is a loop that converts an observed symptom into
a proven root cause and a verified fix. The six phases must run in order:

1. **Reproduce** — Make the bug happen on demand. No reproducible case means no
   fix; you are applying a patch to a system you cannot observe failing.
2. **Isolate** — Narrow the failing behavior to the smallest input, code path,
   and configuration that still triggers it.
3. **Hypothesize** — State, in one sentence, what you believe the root cause is.
   A hypothesis is falsifiable: it predicts what will and will not fix it.
4. **Test** — Confirm or refute the hypothesis with evidence (a print, a log,
   a debugger breakpoint, a minimal test). Do not edit production code yet.
5. **Fix** — Apply the smallest change that addresses the root cause, not the
   symptom.
6. **Verify** — Reproduce the original failure, confirm it no longer occurs,
   and run the wider test suite to catch regressions.

If the fix "works" but you cannot explain *why*, you are back at phase 3, not
done. Write down the causal chain before declaring victory.


## Phase 1 — Reproduce

A bug that cannot be reproduced cannot be verified fixed. Reproduction is the
foundation; spend disproportionate time here.

- Capture the exact inputs: CLI arguments, environment variables, config
  files, input data, Go version (`go version`), OS/arch (`go env GOOS GOARCH`).
- Capture the runtime state at failure: goroutine count, heap size, open file
  descriptors, network connections.
- For timing-dependent bugs, run the case in a loop (`for i := 0; i < 1000;
  i++`) until it fails. Timing bugs that fail 1-in-N runs are still bugs; a
  deterministic repro is preferable but a flaky repro is usable.
- For production bugs, request the logs, metrics, and the deployment version
  that exhibited the failure. Do not debug against a different build than
  shipped.
- Reduce noise: disable unrelated goroutines, shorten input, shrink
  timeouts. The goal is the smallest case that still fails.

Record the reproduction steps as a shell command or test function in your
notes. You will run it again after the fix.

**Failure to reproduce is a finding, not a dead end.** If you cannot reproduce
after reasonable effort, document the conditions you tried and the
environment differences. The bug may be environment-specific (OS, glibc,
network), build-specific (flags, tags), or load-specific (concurrency). Narrow
the dimension that differs.


## Phase 2 — Isolate

Reproduction gives you a failing run; isolation gives you the failing code.

- Use bisection (`git bisect`) to find the commit that introduced the bug.
  Write a script that exits non-zero on failure and let the bisect run.
- Add targeted logging at the boundary between layers (handler → service →
  store). Logs at every line produce noise; logs at boundaries produce signal.
- Comment out or short-circuit half the suspicious code. If the bug persists,
  the cause is in the other half. Repeat (binary search over code paths).
- For panics, the stack trace already isolates the line. For hangs and wrong
  output, you must insert probes.

Isolation is complete when you can name the function and the line where
behavior diverges from expectation. "Somewhere in the parser" is not isolated;
"line 142 returns `nil` when input is empty" is.


## Phase 3 — Hypothesize

State the root cause as a single falsifiable sentence. Good hypotheses:

- "The cache is read after it is invalidated, returning a stale pointer."
- "The goroutine writing to `m` is not synchronized with the goroutine reading
  it, causing a torn read."
- "The channel has no receiver, so the sender blocks forever when the buffer
  fills."

Bad hypotheses (not falsifiable, symptom restatements):

- "It's a race condition." (Which variables? Which goroutines?)
- "The data is wrong." (Wrong how? From where?)
- "It's flaky." (Flaky is a description, not a cause.)

Write the hypothesis down. The act of writing exposes vagueness. If you
cannot write one sentence, you are still in phase 2.


## Phase 4 — Test

Confirm the hypothesis before touching production code. Evidence options,
weakest to strongest:

- **Print / log** — fastest to add, weakest evidence. Useful for confirming a
  value's trajectory through a function.
- **Delve breakpoint** — set a breakpoint at the suspect line; inspect
  locals and the call stack when hit. Confirms runtime state at the moment
  of divergence.
- **Minimal test** — write a test that exercises only the isolated code path
  with the isolated input. If it fails, the hypothesis predicts the failure.
  If it passes, the hypothesis is refuted — return to phase 3.
- **Proof by elimination** — temporarily add a lock, a sleep, or a check that
  the hypothesis says will change behavior. If behavior changes as predicted,
  the hypothesis gains weight. (Remove the experiment before committing.)

A hypothesis is confirmed when you have at least one piece of evidence that
the predicted failure mode occurs under the predicted conditions. Only then
move to phase 5.


## Phase 5 — Fix

Apply the smallest change that addresses the root cause. Symptom-suppression
fixes (catch the panic and return; retry on error; sleep to avoid the race)
are acceptable only as a temporary stopgap and must be marked as such in the
commit message.

- Prefer fixing the cause over adding a guard. A nil-pointer dereference is
  fixed by not storing nil, not by adding a nil check that silently returns.
- If the cause is architectural (shared mutable state across goroutines), the
  fix may be to restructure ownership, not to add a lock.
- Add or update a test that encodes the reproduction case. This test must fail
  without the fix and pass with it. It is your regression guard.
- Run `gofmt -w` on changed files; the format gate rejects unformatted Go.

The fix is incomplete until phase 6 passes.


## Phase 6 — Verify

1. Run the reproduction case from phase 1. It must now pass.
2. Run the new regression test in isolation: `go test -run <NewTest> ./...`.
3. Run the full package suite: `go test ./...`.
4. Run with the race detector: `go test -race ./...` (see below).
5. If the bug was a hang or leak, run under load or for a duration that
   previously triggered it.
6. Re-read the causal chain. If any step is "and then it just works," you
   have a symptom fix, not a root-cause fix. Return to phase 3.

Verification is not "the test passes." Verification is "the test passes AND I
can explain every step of why the bug occurred and why the fix removes it."


# Go-specific tooling

## Delve (`dlv`)

Delve is the Go-native debugger. Prefer it over `gdb` — Go's runtime,
goroutine scheduler, and GC are not transparent to gdb.

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

Common invocations:

- `dlv debug` — debug the program in `main.go`.
- `dlv test ./pkg/...` — debug tests; set breakpoints in test or non-test code.
- `dlv exec ./bin/app` — debug a prebuilt binary.
- `dlv attach <pid>` — attach to a running process (requires permissions).
- `dlv core <binary> <core>` — post-mortem a core dump.

Essential commands inside Delve:

- `break <file>:<line>` or `b <func>` — set a breakpoint.
- `continue` / `c` — run until next breakpoint or panic.
- `step` — step into; `next` — step over; `stepout` — step out.
- `print <expr>` — evaluate an expression in current scope.
- `locals` / `args` — list locals and arguments.
- `goroutines` — list all goroutines; `goroutine <n>` switch; `goroutine <n>
  <cmd>` runs a command on that goroutine.
- `stack` — print the call stack of the current goroutine.
- `frame <n>` — move to frame `n` on the stack.
- `condition <bp> <expr>` — conditional breakpoint; use for loops that fail
  only on iteration N.

When debugging a panic, set a breakpoint *before* the panic line and step
forward; the panic unwinds the stack, so a breakpoint after the panic is never
hit. For a nil dereference, breakpoint on the dereferencing line and inspect
the pointer's origin with `frame` and `print`.

For hangs, `dlv attach <pid>` then `goroutines` and `stack` on each blocked
goroutine reveals where each is parked — typically a channel send/receive or a
lock acquire.


## pprof

`runtime/pprof` and `net/http/pprof` expose CPU, heap, goroutine, mutex, and
block profiles. Use them for performance bugs and goroutine/memory leaks.

CPU profile (batch, finite duration):

```go
import _ "runtime/pprof"
f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()
```

```bash
go tool pprof cpu.prof
(pprof) top
(pprof) list <func>
```

Heap profile (snapshot):

```go
import _ "runtime/pprof"
f, _ := os.Create("heap.prof")
pprof.WriteHeapProfile(f)
f.Close()
```

For long-running services, register the HTTP endpoints:

```go
import _ "net/http/pprof"
go http.ListenAndServe("localhost:6060", nil)
```

Then capture live:

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

- `top` shows the hottest consumers.
- `list <func>` shows source-annotated per-line cost.
- `web` opens an SVG call graph (needs graphviz).
- `-http=:8080` serves a web UI.

Goroutine leak diagnosis: compare `goroutine` counts across snapshots. A
goroutine that appears and never disappears, and whose stack shows it parked
on a channel with no sender, is a leak. Use `full` mode for stack traces:

```bash
curl -s http://localhost:6060/debug/pprof/goroutine?debug=2 > goroutines.txt
```


## Race detector

The Go race detector instruments memory accesses at compile time and reports
unsynchronized concurrent reads and writes of the same address. It is
sound (no false positives) but incomplete (misses races it did not execute).

```bash
go test -race ./...
go build -race -o app . && ./app
```

For long-running services, build with `-race` and run in a staging
environment; the detector reports races as they occur at runtime.

The race detector adds CPU and memory overhead (5-10× typical). Do not run
race-instrumented builds in production for sustained periods.


## Panic and goroutine dump interpretation

### Panic stack trace

A Go panic prints the panic value, then a stack trace from the panicking
goroutine. Read it **bottom-up**: the bottom frame is where the panic
originated; each line above is the caller.

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x48f6a4]

goroutine 1 [running]:
example.com/pkg.(*Server).handle(0xc0000a4000, 0x0, 0x0)
        /work/pkg/server.go:42 +0x244
example.com/pkg.(*Server).Serve(0xc0000a4000, {0x4b3c60?, 0xc0000b8000})
        /work/pkg/server.go:28 +0x85
main.main()
        /work/cmd/app/main.go:15 +0x112
```

- `panic: runtime error: ...` — the panic value; here a nil dereference.
- `goroutine 1 [running]` — the goroutine's state; `[running]` means it was
  executing. Other states: `[chan receive]`, `[semlock]`, `[select]`.
- The first line under `goroutine N` is the **innermost** frame. The `+0xNNN`
  is the PC offset; not usually needed for source-level diagnosis.
- The file:line is the call site within that function. For `server.go:42`,
  open line 42 of `server.go` — the dereference is there or in a function it
  called inline.

For panics wrapped with `fmt.Errorf("...: %w", err)` and re-panicked, the
trace still points to the original panic site; the wrapped message is in the
panic value line.

### Goroutine dump (`SIGQUIT` or `runtime.Stack`)

When a Go program receives `SIGQUIT` (Ctrl+\) or calls `runtime.Stack`, all
goroutines' stacks are printed. Format:

```
goroutine 1 [chan receive]:
main.(*worker).run(...)
        /work/main.go:20 +0x59
created by main.main in goroutine 1
        /work/main.go:10 +0x85
```

- `goroutine N` — the goroutine's ID.
- `[chan receive]` — the state; this goroutine is blocked receiving on a
  channel. Common states: `[chan send]`, `[select]`, `[mutex]`, `[semlock]`,
  `[IO wait]`, `[syscall]`, `[finalizer wait]`, `[GC sweep wait]`.
- `created by ...` — the goroutine that created this one, and the line. Use
  this to trace goroutine leaks back to their origin.

To diagnose a deadlock or hang: capture the full dump, find goroutines in
`[chan ...]`, `[mutex]`, or `[select]` states, and trace each to its creation
site. A cycle of goroutines each waiting on the next is a deadlock.


# Common Go bug patterns

## Nil pointer dereference

Symptom: `panic: runtime error: invalid memory address or nil pointer
dereference`, SIGSEGV.

Common causes:
- A field that was never initialized (zero-value pointer).
- A function returns `(*T, error)` and the caller ignored the error; the
  pointer is nil.
- A map lookup returns the zero value (nil pointer) for a missing key, then
  the nil pointer is dereferenced.

Diagnosis: the stack trace points at the dereference line. Work backward to
find where the nil was assigned or returned. Prefer fixing the assignment
(return a non-nil, or propagate the error) over adding a nil guard at the
dereference site.

## Goroutine leak

Symptom: goroutine count climbs over time (`runtime.NumGoroutine()` or the
`goroutine` pprof endpoint); memory grows; connections are held.

Common causes:
- A goroutine blocks on a channel send/receive whose counterpart has exited.
- A `for range` over a channel whose sender never closes it.
- A `context` was not propagated, so the goroutine never sees cancellation.

Diagnosis: capture `goroutine?debug=2`, find goroutines that persist across
snapshots, read their stack to see what they are blocked on, then read the
`created by` line to find where they were spawned. The fix is to ensure the
channel is closed or the context propagates; goroutines must exit when their
work or context is done.

## Deadlock

Symptom: program hangs; `fatal error: all goroutines are asleep - deadlock!`
(the runtime detects global deadlock and panics).

Common causes:
- A goroutine sends on an unbuffered channel with no receiver.
- A goroutine receives on a channel that is never sent to and never closed.
- Lock ordering inversion: goroutine A holds lock 1 and waits for lock 2;
  goroutine B holds lock 2 and waits for lock 1.
- A `sync.Mutex` or `sync.RWMutex` is locked twice on the same goroutine
  (non-reentrant; use `sync.Mutex` with care, or restructure).

Diagnosis: the runtime deadlock panic prints all goroutine stacks. Find each
blocked goroutine, note the channel or lock, and construct the dependency
cycle. The fix breaks the cycle: add buffering, close the channel, reorder
locks consistently, or restructure to avoid holding a lock across a channel
operation.

Note: the global-deadlock detector only fires when *all* goroutines are
blocked. A deadlock with one goroutine still runnable (e.g., spinning in a
`for {}`) is not detected — use pprof to find it manually.

## Data race

Symptom: `go test -race` reports `DATA RACE`; in production, intermittent
wrong results, torn reads, or crashes under load.

Common causes:
- A map is read and written concurrently from two goroutines without a lock.
  (Maps are not concurrency-safe; this races silently until it corrupts the
  hash table and panics.)
- A slice is appended from two goroutines; the backing array may be shared.
- A `*int` is incremented without `atomic` or a lock.
- A field is read in one goroutine and written in another without
  synchronization (even a single pointer-sized field — the Go memory model
  does not guarantee visibility without a happens-before edge).

Diagnosis: read the race report (below). The fix is to establish a
happens-before edge: a `sync.Mutex`, a channel, or `sync/atomic` for simple
counters. Do not "fix" a race with a sleep or a retry.

## Channel deadlock (specific)

Symptom: hang, or `all goroutines are asleep`.

Common causes:
- Unbuffered channel: sender blocks until a receiver is ready. If the
  receiver goroutine has exited or is itself blocked, the sender parks
  forever.
- Buffered channel: sender blocks when the buffer is full. If the consumer
  is slower than the producer and never drains, the sender parks.
- A `select` with a `default` case never blocks, which can silently drop
  work if the intent was to block.

Diagnosis: in the goroutine dump, find `[chan send]` and `[chan receive]`
goroutines and match senders to receivers. A sender blocked on an
unbuffered channel with no receiver, or a receiver blocked with no sender
and the channel not closed, is the deadlock site.


# Running and interpreting `go test -race`

```bash
go test -race ./...
go test -race -run TestSpecificCase ./pkg/...
```

Always run the race detector on the full suite during CI and before debugging
a suspected concurrency bug. A passing race run does not prove the code is
race-free (the detector only reports races it executes), but a failing run is
definitive — every report is a real bug.

## Reading a race report

```
==================
WARNING: DATA RACE
Read at 0x00c0000160c8 by goroutine 7:
  example.com/pkg.(*Counter).Inc()
      /work/pkg/counter.go:10 +0x44
  example.com/pkg.(*worker).run()
      /work/pkg/worker.go:22 +0x58

Previous write at 0x00c0000160c8 by goroutine 6:
  example.com/pkg.(*Counter).Inc()
      /work/pkg/counter.go:11 +0x64
  example.com/pkg.(*worker).run()
      /work/pkg/worker.go:22 +0x58

Goroutine 7 (running) created at:
  example.com/pkg.(*Pool).Start()
      /work/pkg/pool.go:15 +0x82

Goroutine 6 (finished) created at:
  example.com/pkg.(*Pool).Start()
      /work/pkg/pool.go:15 +0x82
==================
```

- `WARNING: DATA RACE` — header.
- `Read at 0x...` — the address raced on; two goroutines touched the same
  address.
- The two stack traces (`Read ...` and `Previous write ...`) show both
  access sites. Both must be in your code or in standard-library code called
  by your code.
- `created at:` — where each goroutine was spawned; useful when the racing
  goroutines are anonymous (`go func()`).

The fix: synchronize the two accesses. For a counter, `sync/atomic` or a
`sync.Mutex`. For a map, a mutex or `sync.Map` (if the access pattern suits
it). For a struct field shared across goroutines, move it behind a channel or
a mutex-guarded accessor.

Multiple races in one run are reported separately; fix them one at a time and
re-run, because fixing one race can unmask or silence another.


# Logging and tracing

Logging is the first probe; structured logging is the second. Use both to
build a timeline of a failure.

- **Structured logging** — prefer `slog` (standard library) or a compatible
  library. Emit key-value pairs (`slog.Int("items", n)`,
  `slog.String("op", "load")`) so logs are greppable and joinable on a field.
- **Log at boundaries** — handler entry/exit, service call entry, store call
  entry. A log line per line of code is noise; a log per boundary is a
  timeline.
- **Carry a request ID** — generate an ID at the entry point and attach it to
  every log line in the request's context (`slog.With("req_id", id)`).
  Without a request ID, logs from concurrent requests interleave
  untraceably.
- **Log the error, not the message** — `slog.Error("load failed", "err", err)`
  includes the wrapped chain via `err.Error()`. Avoid
  `slog.Error(err.Error())` which discards structure.
- **Levels** — `Debug` for per-iteration detail; `Info` for boundary events;
  `Error` for failures. Resist the urge to log at `Error` for expected
  conditions.

For deeper tracing, Go's `runtime/trace` captures scheduler and GC events:

```go
import _ "runtime/trace"
f, _ := os.Create("trace.out")
trace.Start(f)
defer trace.Stop()
```

```bash
go tool trace trace.out
```

The web UI shows goroutine timelines, blocking durations, syscall time, and
GC pauses. Use it when the bug is scheduling or latency, not correctness.


# Writing a minimal reproduction case

A minimal repro is the most valuable artifact of a debugging session. It:
- confirms the bug exists in isolation (not an interaction with unrelated
  code);
- runs fast, so the fix-test-fix loop is tight;
- becomes the regression test after the fix.

Procedure:

1. Start from the failing production or test scenario.
2. Copy it into a new test function (`func TestReproX(t *testing.T)`).
3. Delete everything not required to trigger the bug. After each deletion,
   run the test; if it still fails, keep deleting. If it stops failing, the
   last deletion removed the trigger — restore it and move on.
4. Replace real dependencies (databases, network) with in-memory fakes or
   interfaces satisfied by a stub. The repro must not require external state.
5. Set a fixed seed for any randomness; remove timeouts unless the bug is
   timing-dependent.
6. The final test should be 20-80 lines. If it is much longer, you have not
   finished isolating — return to phase 2.

A good minimal repro is the proof that you understand the bug. If you cannot
write one, you do not yet understand the bug.


# Root cause analysis techniques

## Five Whys

Ask "why" iteratively until the answer is a process or design gap, not a
person. Example:

1. Why did the service panic? — A nil pointer was dereferenced.
2. Why was the pointer nil? — The loader returned `(nil, nil)` on a missing
   config key.
3. Why did the loader return nil without an error? — The missing-key case
   returned a zero value and nil error.
4. Why does that case return no error? — The loader's contract treats missing
   keys as valid empty configs.
5. Why is that contract acceptable here? — The caller has no way to
   distinguish "configured empty" from "missing," so it cannot guard.

The root cause (item 5) suggests a fix (make the loader distinguish, or make
the caller treat empty as invalid) deeper than "add a nil check at the
dereference."

Stop when the next "why" leaves your system (an external dependency, a human
process outside the code). Five is a guideline, not a limit.

## Bisection (`git bisect`)

When a regression appeared in a known-good codebase, bisect:

```bash
git bisect start
git bisect bad <known-bad-commit>
git bisect good <known-good-commit>
# run your repro; git bisect good or git bisect bad based on the result
git bisect reset
```

For a test-based repro, automate:

```bash
git bisect start HEAD <good-sha> --
git bisect run sh -c 'go test -run TestRepro ./pkg/... || exit 1; exit 0'
```

(Non-zero exit other than 125 marks the commit bad; exit 0 marks it good.
Skip a commit with `git bisect skip` if it does not build.)

Bisection finds the introducing commit in `O(log N)` steps. The commit
message and diff usually reveal the root cause directly. Combine with Five
Whys: the commit explains *what* changed; the whys explain *why* it broke.


# Post-mortem documentation

After the fix is verified, write a short post-mortem. It exists to prevent
recurrence and to transfer the lesson, not to assign blame. Keep it under one
page.

Sections:

- **Summary** — one sentence: what happened and what the impact was.
- **Timeline** — the sequence: detection, reproduction, root cause
  identified, fix merged, verification. Timestamps optional; order is
  mandatory.
- **Root cause** — the causal chain from phase 4, written out. Name the
  function and line. Do not paraphrase; cite the code.
- **Trigger** — the input or condition that activated the latent root cause.
  The bug was usually latent before it was triggered.
- **Fix** — the change applied and why it addresses the root cause (not the
  symptom).
- **What would have caught it earlier** — a missing test, a CI gate, a code
  review checklist item. Convert this into an action item (add the test, add
  the gate).
- **Action items** — concrete, owned, dated: "Add race-detector to CI (owner:
  X, by Y)."

File the post-mortem where the team can find it (a `docs/postmortems/`
directory or an issue tracker). A post-mortem that nobody can find prevents
nothing.


# Checklist (run this before declaring done)

- [ ] Reproduction case written down and re-run after the fix.
- [ ] Hypothesis stated as one falsifiable sentence.
- [ ] Evidence supports the hypothesis (print, debugger, or minimal test).
- [ ] Fix addresses the root cause, not the symptom.
- [ ] Regression test added; fails without the fix, passes with it.
- [ ] `go test ./...` passes.
- [ ] `go test -race ./...` passes (or the race detector is confirmed not
      applicable to this bug).
- [ ] `gofmt -w` applied to changed files.
- [ ] Causal chain written out; no "and then it just works" steps.
- [ ] Post-mortem filed if the bug was user-facing, production-impacting, or
      non-obvious.

If any box is unchecked, return to the corresponding phase. Debugging is
complete when the causal chain is written and the checklist is green — not
when the symptom disappears.
