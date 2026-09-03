---
name: git-workflow
description: Git best practices for branching, committing, merging, rebasing, and resolving conflicts. Use when working with git operations, preparing commits, or resolving merge conflicts.
origin: builtin
argument-hint: "[git task or scope]"
---

`git-workflow` codifies the git practices that keep history readable, merges
safe, and releases reproducible. It is invoked whenever the task touches git:
branching for new work, writing commit messages, staging partial changes,
rebasing vs. merging, resolving conflicts, undoing changes safely, stashing,
cherry-picking, tagging releases, or wiring commits into CI/CD.

Core mental model: **history is a communication artifact, not a log file.**
Every branch, commit message, and merge boundary tells the next reader (a
reviewer, a bisect, a future you, an incident responder) what changed and why.
Optimize for that reader, not for the diff tool.

This skill is advisory. It does not run destructive git operations
(`reset --hard`, `push --force`, `clean -fd`, `branch -D`) without explicit
confirmation, and it never pushes to a remote or rewrites published history on
its own initiative. It explains the operation, states the blast radius, and
waits for the user to confirm.


## Branching strategies

### Feature branches

The default for most teams. One branch per unit of work, branched from the
target integration branch (`main`/`master`/`develop`), merged or rebased back
when done.

- Name branches `<type>/<scope>-<short-desc>`: `feat/auth-oauth`,
  `fix/login-redirect`, `chore/bump-deps`. The prefix makes `git log --oneline`
  and branch lists self-describing.
- Keep branches short-lived (hours to a few days). Long-lived feature branches
  diverge and produce painful merges.
- Rebase onto the target branch frequently (`git fetch && git rebase origin/main`)
  to stay current and shrink the final merge.
- Delete the branch locally and on the remote after merge: `git branch -d
  <branch>` and `git push origin --delete <branch>`. Stale branches accumulate
  and confuse search.

### Git Flow

A heavier model for projects with formal release cycles and long-lived
production branches. Uses five branch types:

- `main` — production-ready, every commit is a release candidate.
- `develop` — integration of completed features for the next release.
- `feature/*` — branched from `develop`, merged back to `develop`.
- `release/*` — branched from `develop`, hardened, merged to `main` and
  `develop`.
- `hotfix/*` — branched from `main`, merged to `main` and `develop`.

Use Git Flow when releases are scheduled and long-lived supported versions
exist. For continuous deployment or trunk-based teams it adds ceremony without
value. Do not adopt it reflexively because a diagram looked authoritative.

### Trunk-based development

Everyone commits to a single long-lived branch (`main`) via short-lived feature
branches or direct commits. Feature branches typically live less than a day.

- Feature flags gate incomplete work so `main` stays deployable.
- Small, frequent commits to `main` over big-bang merges.
- Release branches are cut from `main` only when a release needs stabilization;
  they are short-lived and merged back.
- Best fit for CI/CD-heavy, continuous-deployment environments.
- Pair with trunk-based only if the team commits to small changes and strong
  CI; otherwise `main` destabilizes.

### Choosing a strategy

| Signal | Lean toward |
|---|---|
| Continuous deployment, strong CI, small changes | Trunk-based |
| Scheduled releases, supported older versions | Git Flow |
| Mixed, no formal release cadence | Feature branches + integrate to `main` |
| Solo or small team, fast-moving | Feature branches, rebase-then-merge |

Document the chosen strategy in `CONTRIBUTING.md` or `AGENTS.md` so agents and
contributors branch consistently. Inconsistency here is the most common source
of messy history.


## Commit message conventions

### Conventional Commits

Use Conventional Commits unless the project documents a different convention.
The structure:

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`,
`ci`, `chore`, `revert`. Scope is optional but recommended on multi-module
repos (`feat(auth):`, `fix(ui):`).

Rules:

- Subject line: imperative mood ("add", not "added"/"adds"), lowercase first
  letter, no trailing period, ≤72 characters.
- Body: wrap at ~72 characters, explain *why* (the diff already shows *what*).
  Separate from subject with a blank line.
- Footer: reference issues/tickets (`Closes #123`, `Refs PCB-456`) and note
  breaking changes with `BREAKING CHANGE:` or a `!` after the type/scope
  (`feat(api)!: drop v1 endpoints`).
- One logical change per commit. A commit that touches auth, logging, and the
  build config simultaneously is three commits.

### Semantic Versioning impact

Conventional Commits map to Semantic Versioning bumps:

- `feat:` → minor (`1.2.3` → `1.3.0`)
- `fix:` → patch (`1.2.3` → `1.2.4`)
- `BREAKING CHANGE` / `type!:` → major (`1.2.3` → `2.0.0`)
- `chore:`, `docs:`, `test:`, `build:`, `ci:`, `refactor:` (no behavior change)
  → no version bump.

If the project uses release tooling that derives versions from commit history
(semantic-release, standard-version, release-please), these mappings are
load-bearing — get them right or releases mis-bump.

### When the project has its own convention

Read `CONTRIBUTING.md`, `AGENTS.md`, or recent `git log` before deciding.
Match the existing convention exactly. A repo using `JJ-1234: add login` as
its style should not suddenly receive `feat(auth): add login`. Consistency
beats correctness when the convention is arbitrary but established.


## Staging partial changes

### `git add -p` (interactive patch mode)

The primary tool for splitting a working tree into focused commits. It walks
each hunk and asks `y` (stage), `n` (skip), `s` (split smaller), `e` (edit
hunk), `q` (quit), and more.

- Use it to avoid staging debug prints, temporary comments, or unrelated
  formatting drift into a logical commit.
- `s` splits a hunk into smaller hunks so you can stage part of a function.
- `e` opens the hunk in an editor — manually delete lines prefixed with `-`
  or `+` you do not want staged. Editing context lines (` `) is fragile;
  prefer `s` first.
- After staging, `git commit` commits only staged hunks; `git commit -am`
  commits all tracked changes and bypasses your curation.

### Other staging tools

- `git add <file>` — stage whole files when they belong to one logical change.
- `git add -u` — stage modifications/deletions of already-tracked files only
  (skip untracked).
- `git add -A` — stage everything including new files. Avoid for curated
  commits; useful for sweeping generated artifacts with intent.
- `git add -i` — menu-driven staging; less precise than `-p` but good for
  scanning status.

### Staging review

Before committing, run `git diff --cached` to review exactly what is staged.
This catches accidentally-staged secrets, debug code, and unrelated changes.
Make `git diff --cached` a reflex; it is the single most effective habit for
clean commits.


## Rebasing vs. merging

### When to rebase

- Your feature branch is short-lived and not shared/published. Rebasing moves
  your commits on top of the latest target branch, producing linear history.
- Before opening a PR, `git fetch && git rebase origin/main` to integrate the
  latest and resolve conflicts in small, reviewable pieces.
- When the project's CONTRIBUTING.md mandates linear history.

### When to merge (no-rebase)

- The branch is public/shared (others have commits on it). Rebasing rewrites
  commits others depend on — a destructive operation; see "Safe destructive
  operations".
- You want to preserve the exact branch topology and merge commit as an audit
  trail.
- The project uses merge commits by convention (`--no-ff`).

### Merge strategies

- `git merge <branch>` — default; creates a merge commit if not fast-forwardable.
- `git merge --ff-only <branch>` — merge only if it fast-forwards; otherwise
  fail. Prevents accidental merge commits on `main`.
- `git merge --no-ff <branch>` — always create a merge commit, preserving the
  branch boundary even when a fast-forward was possible. Useful for feature
  branch integration audit trails.
- `git merge --squash <branch>` — collapse the branch's commits into one staged
  change; you then commit once. Loses individual commit messages — only use
  when the intermediate commits are noise (e.g., WIP saves).
- Rebase-then-merge: `git rebase origin/main && git merge --no-ff` — linear
  history plus a merge boundary marker. Common in PR flows.

### Interactive rebase

`git rebase -i <base>` lets you reorder, squash (`s`), edit (`e`), drop (`d`),
and reword (`r`) commits before they land. Use it to clean up a feature branch
before merging. **Only rebase commits that are not yet published.** Rewriting
shared commits is the classic foot-gun.

### Conflict during rebase

Conflicts appear one commit at a time. Resolve each, `git add` the resolved
files, then `git rebase --continue`. Use `git rebase --abort` to bail out
entirely and return to the pre-rebase state. `git rebase --skip` drops the
current commit if it is already applied upstream (rare; understand why before
using).


## Conflict resolution patterns

### Resolution workflow

1. `git status` to see conflicted files (listed under "Unmerged paths").
2. Open each conflicted file; find conflict markers (`<<<<<<<`, `=======`,
   `>>>>>>>`). Understand *why* both sides changed — do not blindly pick one.
3. Edit to the correct final state, removing all conflict markers.
4. `git add <resolved-file>` to mark resolved.
5. `git commit` (merge) or `git rebase --continue` (rebase) to finish.

### Resolution tools

- `git mergetool` — launches a configured visual merge tool. Configure once
  with `git config --global merge.tool <tool>`.
- `git diff --check` — flags whitespace errors and remaining conflict markers
  before you commit.
- `git checkout --ours <file>` / `git checkout --theirs <file>` — accept one
  side wholesale. Use sparingly; wholesale acceptance silently drops the other
  side's intent. Confirm both sides' intent first.
- Three-way view: `git show :1:<file>` (base/common), `:2:` (ours), `:3:`
  (theirs). Useful to inspect each stage in isolation.

### Patterns

- **Semantic resolution** — read the intent of both sides and write the merged
  result that satisfies both, not just text-level union. Two features that
  both add a field to the same struct may both need to be present.
- **Test-driven resolution** — after resolving, run the affected tests. If
  the conflict touched code, the tests are the proof the resolution is sound.
- **Small commits, small conflicts** — frequent rebase/integration shrinks
  conflict surface. A branch 3 days behind `main` conflicts more than one
  integrated hourly.
- **Never commit markers** — `git diff --check` before committing catches a
  leftover `>>>>>>>` that would otherwise ship to `main`.

### Aborting

`git merge --abort` or `git rebase --abort` returns to the pre-conflict state
when resolution goes wrong. Prefer abort-and-rethink over a flailing
half-resolved merge.


## Undoing changes safely

### `git restore` (working tree / index, non-destructive-ish)

- `git restore <file>` — discard working-tree changes to a tracked file.
  Unstaged work is lost; the index is untouched.
- `git restore --staged <file>` — unstage a file (inverse of `git add`).
  Working-tree changes are kept.
- `git restore --source=<ref> <file>` — restore a file from a specific commit
  or branch. Useful to recover a deleted file or pull a known-good version.

`git restore` replaced the older `git checkout -- <file>` and `git reset HEAD
<file>` forms. Prefer it for clarity.

### `git reset` (move a branch pointer; choose mode carefully)

Modes differ by what they touch:

- `git reset --soft <ref>` — moves HEAD; index and working tree unchanged.
  Effect: un-commits, keeps changes staged. Safe-ish.
- `git reset --mixed <ref>` (default) — moves HEAD; index reset, working tree
  unchanged. Effect: un-commits and un-stages, keeps changes in working tree.
- `git reset --hard <ref>` — moves HEAD; index and working tree both reset to
  `<ref>`. **Destructive**: uncommitted changes are lost irreversibly (reflog
  aside).
- `git reset <file>` (no ref) — unstages the file, working tree unchanged.
  Equivalent to `git restore --staged <file>`.

Never run `git reset --hard` over a dirty tree without confirming the
uncommitted work is disposable. It does not go to the stash; it is gone (modulo
reflog, which expires).

### `git revert` (safe history rewrite — adds a commit)

`git revert <ref>` creates a new commit that inverses the changes in `<ref>`.
It does not rewrite history, so it is safe on shared branches. Use it to undo
a commit that has already been pushed.

- `git revert <c1> <c2> ...` — revert multiple commits.
- `git revert <c1>..<c2>` — revert a range.
- `git revert -n <ref>` / `--no-commit` — stage the reversal without
  committing; lets you combine multiple reverts into one commit.

### Decision table

| Want | Safe command |
|---|---|
| Discard unstaged changes to a file | `git restore <file>` |
| Unstage a staged file | `git restore --staged <file>` / `git reset <file>` |
| Undo the last commit, keep changes staged | `git reset --soft HEAD~1` |
| Undo the last commit, keep changes unstaged | `git reset HEAD~1` |
| Undo a pushed commit | `git revert <sha>` (creates a new commit) |
| Discard everything to match a ref | `git reset --hard <ref>` (DESTRUCTIVE — confirm) |

### The reflog is your safety net

`git reflog` records where HEAD and branch tips have been, even after resets
and rewrites. If a `reset --hard` nuked work you needed, check `git reflog`,
find the prior tip, and `git reset --hard <ref>@{1}` (or the listed SHA) to
recover. Reflog entries expire (default ~90 days, ~30 for unreachable), so act
before GC. The reflog is local only — it does not survive a fresh clone.


## Git stash workflow

`git stash` shelves working-tree and staged changes, restoring a clean tree.

- `git stash` / `git stash push` — stash tracked changes (staged and unstaged)
  with a generated message.
- `git stash push -m "wip: auth refactor"` — stash with a descriptive
  message. Always pass `-m`; generated messages are useless later.
- `git stash -u` / `--include-untracked` — also stash untracked files.
- `git stash -a` / `--all` — include ignored files too (rare; usually noise).
- `git stash list` — list stashes as `stash@{0}`, `stash@{1}`, etc.
- `git stash show -p stash@{1}` — inspect a stash's diff before applying.
- `git stash pop stash@{1}` — apply and drop. Conflicts leave the stash in
  the list.
- `git stash apply stash@{1}` — apply without dropping. Use when you want to
  apply the same stash to multiple branches.
- `git stash drop stash@{1}` — drop a stash without applying.
- `git stash clear` — drop ALL stashes. Destructive; confirm first.

### Patterns

- **Context switch without losing WIP** — `git stash push -m "wip: X"` before
  pulling or switching branches, then `git stash pop` after.
- **Stash, pull, pop** — to integrate upstream without a merge commit in a
  dirty tree: `git stash && git pull --rebase && git stash pop`.
- **Partial stash** — `git stash push -m "msg" -- <file> <file>` stashes only
  named paths. Combine with `git add -p` thinking for surgical shelving.
- **Don't treat stash as storage** — stashes are local, expire eventually, and
  are easy to forget. If work needs to persist, commit it to a WIP branch:
  `git switch -c wip/X && git add -A && git commit -m "wip"`.


## Cherry-pick patterns

`git cherry-pick <sha>` applies the diff introduced by a commit onto the
current branch as a new commit.

- **Hotfix propagation** — fix on `main`, then `git cherry-pick <fix-sha>`
  onto `release/*` and `develop` to spread the fix without merging unrelated
  history.
- **Backport** — take a fix from `develop`/`main` onto an older maintained
  release branch.
- **Selective landing** — move one commit from a feature branch without
  landing the rest.

### Flags

- `git cherry-pick <sha>` — apply and commit.
- `git cherry-pick -n <sha>` / `--no-commit` — apply to the index without
  committing; lets you combine multiple cherry-picks into one commit or edit
  before committing.
- `git cherry-pick <c1> <c2> ...` — apply several in order.
- `git cherry-pick <c1>..<c2>` — apply a range (note: `<c1>` excluded).
- `git cherry-pick --abort` — bail out of a conflicted cherry-pick.
- `git cherry-pick --continue` — finish after resolving conflicts.

### Pitfalls

- Cherry-picks create new commits with new SHAs; do not assume the original
  SHA travels with the change. Reference the original in the message
  (`(cherry picked from commit <sha>)`) for traceability.
- Repeated cherry-picks across branches can cause the same logical change to
  appear twice in a future merge, producing "already applied" conflicts. Track
  what has been cherry-picked (a `CHERRY_PICK` log or release notes line).
- Do not cherry-pick merge commits naively; use `git cherry-pick -m 1 <merge>`
  to specify the mainline parent, and verify the result carefully.


## Tag and release management

### Lightweight vs. annotated tags

- `git tag v1.0.0` — lightweight tag; just a named pointer. No metadata,
  no signer, no message. Fine for local/personal markers, poor for releases.
- `git tag -a v1.0.0 -m "Release 1.0.0"` — annotated tag; a full git object
  with tagger, date, and message stored in the object database. **Use
  annotated tags for releases.**
- `git tag -s v1.0.0 -m "Release 1.0.0"` — signed (GPG) annotated tag. Use
  when the project requires verifiable release provenance.
- `git tag -l "v1.*"` — list tags matching a pattern.
- `git push origin v1.0.0` — push a single tag. Tags are not pushed by default.
- `git push origin --tags` — push all tags (rarely what you want for curated
  releases).
- `git tag -d v1.0.0` — delete a local tag.
- `git push origin :refs/tags/v1.0.0` / `git push origin --delete v1.0.0` —
  delete a remote tag.

### Release workflow

1. Ensure the release branch is at the intended commit (CI green, changelog
   updated, version bumped).
2. `git tag -a v<version> -m "Release <version>"` from the release commit.
3. `git push origin v<version>` to publish the tag.
4. Trigger release tooling/CI from the tag, or cut release artifacts from it.
5. Update the changelog with the tag's commit and date.

### Semantic versioning

Tag releases `vMAJOR.MINOR.PATCH` (`v1.2.3`). Follow SemVer:

- MAJOR — incompatible API changes.
- MINOR — backward-compatible feature additions.
- PATCH — backward-compatible bug fixes.

Pre-release tags: `v1.0.0-rc.1`, `v1.0.0-beta.2`, `v1.0.0-alpha.1`. Build
metadata: `v1.0.0+exp.sha.4f8a`. Tag the release commit, not the tip of
`main`, so the tag points at exactly what was shipped even as `main` moves on.


## Safe destructive operations

Operations that can permanently lose work. This skill explains them and their
blast radius, and waits for explicit user confirmation before running any.

### Never run without confirmation

- `git reset --hard <ref>` — discards uncommitted work in index and working
  tree.
- `git clean -fd` / `git clean -fdx` — deletes untracked files (and, with
  `-x`, ignored files). Untracked work has no reflog; it is gone.
- `git checkout .` / `git restore .` / `git checkout -- .` — discards
  working-tree changes broadly.
- `git branch -D <branch>` — force-delete an unmerged branch.
- `git push --force` / `git push -f` — rewrites the remote ref, discarding
  others' commits.
- `git push --force-with-lease` — safer force push (fails if remote moved),
  but still destructive to unpushed local commits if misapplied.
- `git stash clear` — drops all stashes with no undo.
- `git rebase` / `git commit --amend` / `git rebase -i` on commits that have
  been pushed — rewrites shared history.
- `git filter-branch` / `git filter-repo` — rewrites history across the whole
  repo (secrets, large files). Coordinate with every collaborator; force-push
  and have everyone re-clone.

### Force-push safety ladder

When a force-push is genuinely needed (e.g., cleaning a private feature
branch after interactive rebase):

1. Confirm the branch is not shared. `git branch -r --contains <branch>` and
   check for other contributors.
2. Prefer `git push --force-with-lease` over `--force`. It refuses to push if
   the remote has advanced, preventing you from clobbering others' commits.
3. `git push --force-with-lease=<branch>:<expected-sha>` — the strictest
   form; only push if the remote is still at the SHA you expect.
4. Never force-push to `main`/`master` or a shared integration branch. Protect
   these branches at the host (GitHub/GitLab branch protection) as a backstop.

### Protected branches

Treat `main`/`master` and `develop` (or equivalent) as protected: no
force-push, no direct commits without review where review is required, no
history rewrite. Branch protection rules at the host are the enforcement
layer; this skill honors them by refusing to rewrite published history unless
the user explicitly overrides with confirmation.

### Recovery before destruction

Before any destructive op, surface the recovery path: `git reflog`,
`git fsck --lost-found`, and the fact that `--hard`/`clean` leave nothing to
recover. State plainly: "this will discard uncommitted work that is not in any
commit, branch, or stash — confirm it is disposable."


## Integration with CI/CD

### Commit-driven CI

Most CI systems trigger pipelines on `push` and/or `pull_request` events.
Conventions that keep CI useful:

- Small, focused commits produce focused CI runs and focused failures.
  Monolithic commits make a red pipeline un-diagnosable.
- Conventional Commit types let CI route jobs: run full test suite on
  `feat`/`fix`, lint-only on `docs`/`chore`.
- `[skip ci]` or `[ci skip]` in a commit message skips CI for that commit when
  the change is documentation-only and does not need verification. Use
  sparingly — CI is the gate; skipping it should be intentional and reviewed.
- Keep `.github/workflows/`, `.gitlab-ci.yml`, or equivalent under version
  control and reviewed like any other code.

### Branch-based CI

- Run fast checks (lint, unit tests) on every push to a feature branch.
- Run the full suite (integration, e2e) on PRs and on the target branch.
- Use `--ff-only` merges or squash merges in CI integration to keep history
  linear and pipelines deterministic.
- Cache dependencies (`~/.cache`, Go module cache, npm cache) keyed on lockfile
  hash to keep CI fast without staleness.

### Release-driven CI

- Cut a release by tagging (`git tag -a vX.Y.Z`); CI detects the tag and
  builds/publishes artifacts. Tag-driven releases are reproducible — the tag
  points at the exact commit shipped.
- Derive release notes from Conventional Commits via tooling
  (release-please, semantic-release) when the convention is followed.
- Never publish from an untagged tip; the tag is the auditable release
  boundary.

### Secret hygiene

- Never commit secrets. CI scanners (e.g., gitleaks, trufflehog) on every
  push catch leaked credentials.
- If a secret is committed, treat it as compromised: rotate it, remove it
  from history only if it has not been cloned/fetched (otherwise the rewrite
  is theater — the secret is already out), and add it to CI scanning to
  prevent recurrence. Rewriting history does not revoke a leaked credential.


## Pre-commit hygiene checklist

Before finalizing any commit, run through this:

- `git status` — confirm only intended files are staged; no `bin/`, build
  outputs, or generated artifacts that should be ignored.
- `git diff --cached` — review the exact staged diff; confirm no debug
  prints, secrets, or unrelated formatting.
- `git diff --check` — catch whitespace errors and leftover conflict markers.
- Commit message: correct type/scope, imperative subject, ≤72 chars, body
  explains why, footers reference tickets.
- One logical change per commit. If the diff spans unrelated concerns, split
  it with `git reset` + `git add -p` into multiple commits.
- Hook hooks: if a `pre-commit` hook reformats files, re-stage and amend the
  commit so the formatted output is part of the commit, not a follow-up.


## Decision shortcuts

| Situation | Default action |
|---|---|
| Start new work | `git switch -c feat/<scope>-<desc>` from latest `main` |
| Save WIP to context-switch | `git stash push -m "wip: <desc>"` |
| Integrate upstream into feature | `git fetch && git rebase origin/main` |
| Finish a feature | rebase, then merge `--no-ff` or squash per project convention |
| Undo last local commit, keep work | `git reset --soft HEAD~1` |
| Undo a pushed commit | `git revert <sha>` |
| Discard all local changes | `git reset --hard` (confirm first) |
| Move one commit between branches | `git cherry-pick <sha>` |
| Cut a release | `git tag -a vX.Y.Z -m "Release X.Y.Z"` + push tag |
| Resolve a conflict | edit to semantic correctness, `git add`, `git rebase --continue` / `git commit` |
| Recover destroyed work | `git reflog`, then `git reset --hard <sha>` |

When the situation does not fit a shortcut, fall back to the principles:
history is for the reader, never rewrite published history, never destroy
uncommitted work without confirmation, and prefer safe inverses (`revert`,
`restore --staged`) over destructive resets.
