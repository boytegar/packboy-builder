---
name: code-review
description: Review code changes and identify high-confidence, actionable bugs. Use when the user wants to review a pull request or branch diff, find bugs, security issues, or correctness problems in code changes.
origin: builtin
---


`/code-review` reviews a pull request, branch diff, or arbitrary set of code
changes and produces a structured, severity-ranked list of **high-confidence,
actionable** findings. It is a bug finder, not a linter: it reports issues that
would cause incorrect behavior, security exposure, data loss, or measurable
performance regression in production. Style nits, subjective taste, and
formatting are out of scope unless they mask a real defect.

The review is grounded in the actual diff and the surrounding code — never in
guesses about what the code "probably" does. Every finding cites a `file:line`
that exists in the change or its immediate context, and every recommendation is
specific enough to apply directly without re-deriving the analysis.

This skill is stack-aware but defaults to Go because Packboy Builder (`pcb`)
is a Go CLI/TUI. When the diff touches another language, apply the
language-specific sections in Phase 4 and Phase 6 and skip the Go-only
anti-patterns that do not apply.


Run the phases in order. Do not skip Phase 1 (context) — reviewing a diff
without knowing what the change is *for* produces false positives that waste
the author's time. Do not skip Phase 5 (triage) — a review that reports 40
low-value findings is worse than no review, because the signal drowns in noise.


## Phase 1 — Establish change context

Before reading any code, establish what changed and why. The "why" comes from
the commit history and PR description; the "what" comes from the diff itself.

1. **Identify the base and head.** Determine the merge base so the diff shows
   only what this branch introduced, not everything since `main`'s initial
   commit. Prefer the three-dot diff:
   ```bash
   git fetch origin
   git merge-base origin/main HEAD        # prints the base SHA
   git diff origin/main...HEAD --stat     # file-level summary
   git diff origin/main...HEAD           # full diff
   ```
   For a single-commit review, use `git show <sha>`. For staged-but-uncommitted
   work, use `git diff --cached`.

2. **Read the commit log for intent.**
   ```bash
   git log origin/main..HEAD --oneline
   git log origin/main..HEAD --format="%h %s%n%b" -- <changed-paths>
   ```
   The commit messages tell you the author's goal. A change titled "fix race in
   session save" narrows your focus to concurrency around that path; a change
   titled "refactor agent dispatch" widens it to the dispatch surface. If the
   PR description is available, read it for the stated goal and the "before"
   behavior.

3. **Size up the blast radius.** From `--stat`, note:
   - Files with large net additions (> +300 lines) — these carry the most risk
     and deserve line-by-line attention.
   - Files that are pure deletions — usually safe, but confirm no caller
     depended on removed exported symbols.
   - Generated or vendored files (`*.pb.go`, `*_stringer.go`, `vendor/`,
     `bin/`) — skip these unless the generation source is also in the diff and
     hand-edited, which is a smell itself.
   - Test files vs. source files — a large source change with zero test
     additions is a flag for Phase 7.

4. **Detect mechanical mass-edits.** If `--stat` shows the same handful of
   lines changed across dozens of files (a rename, a lint auto-fix, a
   version bump), review the *pattern* once and sample two or three files to
   confirm it was applied consistently. Do not review each file individually —
   that is where review fatigue hides real bugs in adjacent, non-mechanical
   edits. Instead, isolate the non-mechanical hunks and focus there.


## Phase 2 — Read the changed code in full context

A diff hunk shows only the changed lines and a few lines of context. Many bugs
live in the *interaction* between the changed lines and the unchanged code
around them, so you must read the full function, not the hunk.

1. For each changed file, open the full file (or at least the full enclosing
   function). Note:
   - The function signature and its callers (who passes what in).
   - The struct/method the change lives in — does it have documented
     invariants the change might violate?
   - The package's existing error conventions (sentinel errors? `errors.New`?
     wrapped with `%w`? logged-then-discarded?).

2. Use `git log -p -L <start>,<end>:<file>` to see the history of the specific
   changed region. A line that looks suspicious in isolation may be a
   deliberate workaround for a prior bug; the history tells you which.

3. For removed code, search the whole repo for callers:
   ```bash
   git grep -n '<removed-symbol>'
   ```
   If the symbol was exported and the repo has no other callers, a downstream
   consumer (not in this repo) may still depend on it. Flag removals of
   exported API as at least a `warning` unless the change is explicitly a
   breaking change.

4. Read the tests that accompany the change, even if you do not run them
   (running is Phase 7). Tests reveal the author's mental model of the
   behavior; a test that asserts `nil != err` where the code now returns `nil`
   is a bug you would miss from the source alone.


## Phase 3 — Bug-category sweep

Go through the changed code once per category. Reviewing category-by-category
beats reading top-to-bottom because each pass has a single hypothesis ("is
there a race here?") and you do not lose focus by context-switching between
concerns. The categories below are ordered roughly by how often a real bug
falls in them; adjust per the diff.

### 3.1 Correctness

Look for logic that does not do what the author intends or what the caller
expects.

- **Off-by-one and boundary errors.** Loop bounds, slice indexing, `<=` vs
  `<`, `len(x)` vs `len(x)-1`, empty-slice vs nil-slice handling. In Go,
  `s[len(s)]` panics; `s[len(s):]` is valid and yields an empty slice.
- **Nil checks in the wrong place.** A `nil` check before a dereference that
  is actually unguarded elsewhere on the same value; a nil-check that was
  needed and is now gone after a refactor.
- **Shadowed return values.** `err` declared with `:=` inside a block that
  shadows the named return, so the outer error is never set:
  ```go
  func f() (err error) {
      if x, err := compute(); err != nil { // shadows the named return
          return err
      }
      _ = x
      return nil // err from compute() is lost
  }
  ```
- **Incorrect type assertions.** `v.(T)` without the `, ok` form panics on a
  mismatch; prefer `v, ok := x.(T)`.
- **Map iteration order assumptions.** Go randomizes map iteration; code that
  depends on order is a latent bug, not a stylistic one.
- **Goroutine capture of loop variables.** Pre-Go-1.22 semantics shared one
  variable per loop; confirm the project's `go.mod` `go` directive before
  deciding whether `for _, x := range { go f(x) }` is safe. When unsure, treat
  a captured loop variable in a goroutine as a `warning` and verify the
  toolchain version.
- **Comparison bugs.** Comparing pointers to strings/structs with `==` when
  value equality was intended; comparing `time.Time` with `==` across
  timezones or after monotonic-clock stripping.

### 3.2 Error handling

- **Swallowed errors.** `_, _ = f()`; `_ = err`; `if err != nil { return }`
  with no wrapping or log; `defer f.Close()` ignoring the returned error when
  that error matters (writes, transaction commit).
- **Wrapping that loses the sentinel.** `fmt.Errorf("x: %v", err)` (uses `%v`,
  not `%w`) breaks `errors.Is` for callers that check for a sentinel. Flag
  when the caller visibly relies on `errors.Is` or `errors.As`.
- **Returning `nil` after a partial failure.** A function that allocates
  resources, hits an error, returns `nil`-error, and leaves the resources
  leaked or half-initialized.
- **Error messages that leak internals.** Returning raw SQL or file paths to
  end users in a CLI/TUI context where they may be surfaced.
- **Panics used for control flow.** `panic` in a library package where the
  caller cannot recover; a `recover` that swallows the panic silently.

### 3.3 Race conditions

- **Shared mutable state across goroutines without synchronization.** A field
  written in one goroutine and read in another with no mutex, channel, or
  `sync/atomic`. In a Bubble Tea TUI (this project), the `Model` is updated on
  the main loop but background goroutines that mutate model fields directly
  are a classic race — they must send `tea.Msg` instead.
- **`sync.WaitGroup` misuse.** Calling `wg.Add(1)` inside the goroutine
  instead of before `go f()`; the caller's `wg.Wait()` can fire before `Add`.
- **Channel closure by the wrong side.** Closing a channel from the receiver
  or from multiple senders panics. The sender owns the close.
- **`sync.Mutex` copy.** Passing a struct containing a mutex by value copies
  the mutex — `go vet` catches this; flag if `vet` clearly was not run.
- **Check-then-act on shared state.** `if m[k] == nil { m[k] = new() }` is a
  race even under a read lock; the write needs the write lock.

### 3.4 Resource leaks

- **Unclosed `io.Closer`.** `os.Open`, `os.Create`, `sql.Rows`, `http.Body`,
  `gzip.Reader` opened without `defer Close()`. For files opened in a loop or
  branch, confirm `Close` is reached on every path.
- **Goroutine leaks.** A goroutine blocked forever on a channel whose sender
  has returned, or on a `context` that is never canceled. The goroutine and
  anything it pins (closures, large buffers) never get collected.
- **`time.Ticker`/`time.Timer` not stopped.** `t := time.NewTicker(...)`
  without `defer t.Stop()` leaks the timer.
- **HTTP response bodies.** Even on error, `resp.Body.Close()` must run; the
  common pattern is `defer resp.Body.Close()` immediately after the nil-err
  check, before reading the body.
- **Transaction not finalized.** `db.Begin()` without `Commit` or `Rollback`
  on every path; a `defer tx.Rollback()` that is correct, but a missing
  `tx.Commit()` on the happy path that leaves the rollback to fire.

### 3.5 Security

See the dedicated checklist in Phase 6. Flag here only the highest-severity
items so Phase 3 stays a sweep.

### 3.6 Performance

See Phase 6's performance section. During the sweep, note only gross issues:
O(n²) over a growing dataset inside a hot path, needless allocations in a
loop, blocking I/O on the main/UI goroutine.

### 3.7 API and contract

- **Changed function signatures.** An added required parameter, a removed
  return value, a changed exported type — these break callers. Even within a
  repo, flag if the change is in `internal/core` or another widely-imported
  package per the dependency rules.
- **Changed behavior without a changed signature.** A function that used to
  return `(value, nil)` for a missing item and now returns `(zero, error)` —
  every caller's semantics shifted silently.
- **Narrowed input validation.** A function that previously rejected empty
  input and now accepts it, or vice versa, without updating callers/tests.

### 3.8 Concurrency-correctness of channels and selects

- **`select` with a `default` that silently drops work** when the author
  intended blocking.
- **`select` that can starve.** A `select` with two ready cases where one
  must always win; Go picks randomly, which may not match intent.
- **Sending on a `nil` channel blocks forever.** A channel that is `nil`-on-a
  field that is conditionally initialized, sent to in a select — the case
  never fires (blocks), which may or may not be intended.


## Phase 4 — Stack-specific anti-patterns

Apply the Go anti-patterns below. If the diff is not Go, substitute the
equivalent set for that language and skip this subsection.

### Go anti-patterns to flag

1. **`interface{}`/`any` where a concrete type is clearer.** Especially in
   exported APIs; it pushes type-checking to runtime.
2. **Returning `error` from a `Stringer` or `Marshal*` method.** These are
   not allowed by their interfaces; the code will not compile or will panic.
3. **`init()` with side effects.** Ordering across packages is
   non-deterministic; side effects (opening files, starting servers) belong in
   `main` or an explicit `Setup`.
4. **Capturing a loop variable in a closure** (pre-1.22). See 3.1.
5. **`math/rand` for security.** `math/rand` is not cryptographic; flag if used
   for tokens, IDs, or anything an attacker could predict.
6. **`time.Sleep` instead of a context-aware wait.** A sleep that cannot be
   canceled extends startup/shutdown latency under load.
7. **`fmt.Sprintf` in a hot path.** Allocates; prefer `strconv.Itoa` or
   pre-formatted constants when the format string is static.
8. **`append` to a slice aliased into another slice.** `y := append(x[:i:i],
   v)` is safe; `y := append(x, v)` can mutate the backing array of `x`'s
   neighbors. Flag when two slices share a backing array and one is appended.
9. **`string([]byte)` in a loop.** Each iteration allocates; build with
   `strings.Builder`.
10. **Global mutable state** (`var x = ...` at package scope mutated by
    functions) that is read across goroutines — a correctness bug (3.3) and a
    testability smell.
11. **`context.Background()` inside a library.** A library should accept a
    `context.Context` and let the caller cancel; injecting `Background()` makes
    the call uncancellable from above.
12. **Errors constructed with `errors.New` at call sites** when a sentinel
    exists for that condition and the caller checks it — callers cannot match.

### Other languages (when present in the diff)

- **SQL (any driver):** string-concatenated queries (injection), missing
  `LIMIT` on user-driven queries, `SELECT *` feeding into struct scans that
  will silently break on a column add.
- **Shell scripts:** unquoted variables (`$VAR` vs `"$VAR"`), `eval` on
  untrusted input, `cd` without `set -e`/`&&` chain leaving later commands
  running in the wrong directory.
- **YAML/JSON configs:** secrets committed in plaintext, `latest` image tags
  in production manifests, host-network mode in a container that does not
  need it.


## Phase 5 — Triage and severity

Assign each candidate finding a severity. The goal of triage is to **drop**
low-value candidates, not to inflate the count. A review that reports three
real bugs is trusted; one that reports thirty guesses is ignored.

- **critical** — Exploitable security issue, data loss, or a crash reachable
  from normal operation. Merge should not proceed until fixed or explicitly
  accepted by an owner.
- **high** — Likely-wrong behavior in a real path, a resource leak that
  accumulates, or an API break. Should block merge unless the scope is
  explicitly experimental.
- **medium** — A real defect with limited blast radius (an edge case the
  tests miss, a perf issue on a warm path), or a correctness issue the author
  can trivially confirm is non-impacting. Fix before merge preferred.
- **low** — Minor, localized, and low-impact; a `defer` on a path where the
  leak is bounded, a cosmetic error message. Mention but do not block.
- **nit / question** — Not a bug; a question to confirm intent or a suggestion.
  Keep these to a minimum; if you have more than a handful, the review is
  drifting into style.

**Drop the finding if any of these hold:**
- You cannot point to a `file:line` that demonstrates the defect.
- The "bug" requires input the code explicitly cannot receive (unreachable
  from any caller in the repo or documented contract).
- The "bug" is a style preference (gofmt would not change it; it is taste).
- The "bug" is already guarded by a check you initially missed — re-read before
  reporting.

When in doubt about reachability, do not assume. Use `git grep` for callers;
if you cannot establish reachability, downgrade severity and label the finding
"reachability: unverified".


## Phase 6 — Deep checks

### Security checklist

Run through this list for every change that touches input parsing, network,
auth, files, or external process invocation.

- **Injection.**
  - SQL: parameterized queries only; no `fmt.Sprintf` into a query string;
    no `+` concatenation of user input into SQL.
  - Shell: no `exec.Command("sh", "-c", userInput)`; prefer argument lists.
  - Path: user-supplied paths joined without cleaning/containment check
    (`filepath.Join(root, userInput)` that can escape `root` via `..`). Use
    `filepath.Rel` + a prefix check, or a dedicated safe-join.
  - Template/HTML: user input rendered without escaping.
- **Authentication & authorization.**
  - A check removed or weakened (`if isAdmin` flipped to `if true`; a
    `Permission` field no longer consulted).
  - Tokens/credentials compared with `==` (timing side-channel) instead of
    `subtle.ConstantTimeCompare`.
  - A new endpoint or command that performs a privileged action with no auth
    gate added.
- **Data exposure.**
  - Secrets, tokens, or passwords logged at any level, or returned in error
    messages.
  - PII or full request bodies written to a log file the operator did not opt
    into.
  - A `debug`/`verbose` flag that prints credentials or full env vars.
  - Credentials committed to the repo (`.env`, `config.yaml`, test fixtures
    that are not clearly `example`).
- **Unsafe deserialization.** `gob`, `json` into `interface{}` then
  type-asserted, or any decoder that can instantiate arbitrary types from
  untrusted input.
- **Crypto.** Custom crypto, `math/rand` for security, ECB mode, hardcoded
  keys/IVs, or a nonce reused under the same key.
- **Resource exhaustion from untrusted input.** No `MaxBytesReader` on an
  HTTP body, no size cap on a decode, no cap on a goroutine fan-out per
  request.

### Performance checklist

- **N+1 queries.** A query inside a loop over a result set; a per-item
  lookup where a batch `IN` or join would do.
- **Unnecessary allocations in hot paths.**
  - `string(b)` then `[]byte(s)` round-trips.
  - `append` in a loop where `make([]T, 0, n)` with a known `n` avoids
    reallocations.
  - `fmt.Sprintf` where a constant or `strconv` suffices.
  - Boxing into `interface{}` (and back) in a tight loop allocates.
- **Goroutine leaks.** See 3.4 — a leaked goroutine is also a perf bug
  because it pins memory.
- **Blocking calls on the UI/main goroutine.** In a Bubble Tea app, any
  `time.Sleep`, file I/O, or network call on the update loop freezes the
  interface; it must run in a `cmd` and return a `tea.Msg`.
- **O(n²) or worse on a growing input** where a map or sort would make it
  O(n log n) or O(n). Flag when `n` is unbounded (user-driven) rather than
  bounded by a small constant.
- **Repeated work** that could be memoized or hoisted out of a loop.
- **Unbounded buffers.** A channel with no cap fed by an unbounded producer;
  memory grows until OOM.

### Race-condition deep check

If the change adds goroutines, channels, or shared state:

- Identify every shared variable and confirm each has exactly one
  synchronization strategy (mutex, atomic, channel, or single-goroutine
  ownership). Two strategies on the same variable is a smell.
- Confirm `context` cancellation propagates to every goroutine the change
  starts; a goroutine that ignores its context will outlive a shutdown.
- For `select` blocks, confirm the `default` case (if present) is intended
  to be non-blocking, and that a missing `ctx.Done()` case is deliberate.


## Phase 7 — Test coverage verification

A change to application logic without a test is a finding in its own right.
For logic-only changes (UI wiring, mechanical refactor, docs), tests may not
be required — use judgment.

1. **Identify the changed behavior's contract** — the set of inputs and
   expected outputs (or side effects) the new code claims to implement.
2. **Locate the tests.** `git grep -n '<func-name>' -- '*_test.go'`. Note
   whether each branch of the new code is exercised.
3. **Run the relevant tests** if the environment allows:
   ```bash
   go build ./...
   GOCACHE=/tmp/gocache go test ./...
   # or, scoped:
   GOCACHE=/tmp/gocache go test ./internal/skill/... ./internal/subagent/...
   ```
4. **Assess coverage, not line count.** A test that sets up the exact
   preconditions for the bug, asserts the postcondition, and would fail if the
   bug were present is high-value. A test that calls the function and asserts
   `no error` is low-value and may hide the bug.
5. **Flag missing edge tests** explicitly: "no test for the empty-input case
   the new branch claims to handle" — this is often the most actionable
   finding for the author because it tells them exactly what to add.
6. **Do not insist on 100% coverage.** Error paths that are genuinely
   unreachable from any real caller do not need a test; chasing them
   encourages tests that exist only to inflate the number.


## Phase 8 — Structured output

Produce the review as a single structured document. Every finding has exactly
these fields; omit none. Use the severity ordering from Phase 5, highest
first.

```
## code-review — <branch or PR ref>

Scope: <files changed count> files, <+added/-removed> lines
Base: <merge-base SHA or ref>
Reviewed: <today's date>

### Summary
<2-5 sentences: what the change does, overall risk level, and whether you
recommend merge / merge-with-fixes / block. Name the single most important
finding up front so a busy reader gets the headline.>

### Findings

#### [critical] <short title> — <file>:<line>
Description: <what is wrong, grounded in the code. One paragraph.>
Impact: <what breaks if shipped, concretely. Who/what is affected.>
Recommendation: <the specific fix. Name the function, the line, and the
change. "Wrap with %w" / "add defer Close() on line N" / "move wg.Add(1)
before the go statement".>
Reachability: <verified via git grep / inferred / unverified>

#### [high] <short title> — <file>:<line>
...

### Coverage
<per changed behavior: does a test exist? one line each. Be honest — "no
test for the empty-input branch" is a finding, not a complaint.>

### Notes
<optional: questions to the author, non-blocking observations, and
anything you could not verify. If empty, omit the section.>
```

Rules for the output:

- **Cite real `file:line`.** Every finding's location must be a line that
  exists in the diff or its immediate context. If the issue is structural
  (a missing check across the whole function), cite the function's first line.
- **One finding per heading.** Do not bundle. A reader needs to resolve each
  independently.
- **No findings? Say so explicitly.** "No high-confidence defects found.
  Verified <list what you checked>." Silence is indistinguishable from
  "did not review."
- **No speculative future risk.** "This might be slow at scale" without a
  measurement or a concrete input that triggers it is not a finding. Drop it
  or move to Notes.
- **Recommendations are actionable.** "Improve error handling" is not
  actionable. "On line 42, wrap the error with `fmt.Errorf(\"load config:
  %w\", err)` so `errors.Is(err, ErrMissing)` works at the call site" is.
- **Keep the Summary honest.** If you did not run the tests because the
  environment could not, say "tests not run (env unavailable)" rather than
  implying they passed.


## Distinguishing real bugs from style

The line between a bug and a taste preference is the most common source of
review noise. Use this decision rule:

- **It is a bug if** the behavior diverges from the author's stated or
  implied intent, from the function's documented contract, or from what a
  reasonable caller would expect — and the divergence manifests on an input
  the code can actually receive.
- **It is a style issue if** the code is correct and you merely would have
  written it differently (different name, different structure, different
  abstraction).
- **It is a latent bug if** the code is correct today but a trivial future
  change (a new caller, a larger input, a nil where a value was assumed)
  turns it into a bug. Report latent bugs at `low` or `medium`, labeled
  "latent", only when the trigger is plausible.

When a finding could be either, state both readings and let the author
decide. Do not escalate a style issue to a bug by framing — the severity must
reflect the actual defect, not your enthusiasm for the rewrite.

Examples of what NOT to report as a bug:
- A function is longer than you would write.
- A name is less descriptive than you would choose.
- A comment is missing on a line you find non-obvious but that any reader of
  the language would understand.
- The author used a `for` loop where a `range` would work.

Examples of what IS a bug, even if small:
- A `defer` is on the wrong line so it does not run on the error path.
- An error is wrapped with `%v` where the caller uses `errors.Is`.
- A map is read and written without a lock from two goroutines.
- A `select` drops a message because of an unintended `default`.


## Do

- Ground every finding in a `file:line` you can point to in the diff or its
  immediate surrounding code.
- Re-read the full function, not just the hunk, before reporting a logic
  bug — the hunk often omits the guard that makes the change safe.
- Verify reachability with `git grep` before assigning `critical`/`high`; an
  unreachable defect is `low` at most.
- Prefer a single high-signal finding over ten plausible-sounding ones.
- State what you checked, even when you found nothing, so the reader knows
  the review's scope.
- Run the tests (or state that you could not) — do not imply a pass you did
  not observe.
- Tailor Go-specific checks to the project's `go.mod` toolchain version;
  behaviors like loop-variable capture changed across versions.
- Use the project's own verification commands (`go build ./...`, `go test`) rather than inventing equivalents.


## Don't

- Report a bug you cannot localize to a `file:line`.
- Dress a style preference as a correctness issue to make it sound urgent.
- Report every `err` that is not wrapped — only flag when a caller relies on
  `errors.Is` or `errors.As` for that specific error.
- Insist on tests for pure UI wiring, mechanical renames, or doc-only changes.
- Run long commands or start dev servers as part of the review; this is a
  read-and-verify skill, not a runtime check.
- Modify the code under review. This skill reports findings; it does not
  apply fixes. If the author wants the fix applied, that is a separate task.
- Assume the diff reflects intent perfectly — commit messages and PR
  descriptions are the intent source; the diff is the implementation. A
  mismatch between the two is itself a finding.
- Block merge on `low`/`nit` findings; reserve block recommendations for
  `critical`/`high` where the defect is established and reachable.
- Report a performance issue without naming the input that triggers it or the
  complexity class; "could be slow" is not a finding.
- Duplicate findings across categories. If a race is also a resource leak,
  report it once under the higher-severity category and cross-reference.


## Verification commands (reference)

Keep these handy to keep output readable.

```bash
# What changed
git fetch origin
git diff origin/main...HEAD --stat
git diff origin/main...HEAD
git log origin/main..HEAD --oneline

# History of a specific region
git log -p -L <start>,<end>:<file>

# Callers of a changed/removed symbol
git grep -n '<symbol>'

# Build and test (Go)
go build ./...
GOCACHE=/tmp/gocache go test ./...
GOCACHE=/tmp/gocache go test ./internal/skill/... ./internal/subagent/...
```

Scope test runs to the packages the change touches when the full suite is
slow; note the scope in the output so the reader knows what was covered.
