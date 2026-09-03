---
name: wiki-generation
description: Generate comprehensive codebase documentation for a repository. Produces structured wiki covering architecture, packages, API, and workflows.
origin: builtin
---


Generate a comprehensive, accurate, and navigable wiki for a codebase. The wiki
is not a restatement of the README — it is a structured reference derived by
reading the actual source, manifests, and tests. Every claim traces back to a
`file:line` reference or is explicitly marked as inferred.

The wiki is a product, not a byproduct. Quality comes from the phase order:
discover the real structure BEFORE analyzing, analyze the actual code BEFORE
documenting, and verify the documentation against the code BEFORE shipping.
Never document what you did not read.


- Generating a wiki for a repository the first time.
- Refreshing a stale wiki after significant changes.
- Producing structured documentation for onboarding, handoff, or audit.
- Generating reference docs for a Go module, CLI, or TUI codebase.

- You are a DOCUMENTATION WRITER grounded in evidence, not an implementer.
  You do not modify source files, run builds, or change configuration.
- The only files you create are the four wiki files under the target output
  directory (default: `wiki/` at the repository root).
- Every factual claim about the codebase must be traceable to a `file:line`
  reference. If you cannot cite a source, mark the claim as **(inferred)**.
- Read the actual source. Do not infer package responsibilities from names
  alone. Do not invent types, functions, or dependencies that you did not
  observe by reading.
- Prefer a `researcher` subagent (mode=explore) when discovery touches more
  than 2-3 packages, to keep your own context lean and the output consistent.
- All output is in English regardless of the repository's comment language.

---

Run the four phases in order. For small repositories phases 1-2 collapse into
one pass, but never skip phase 1 (discover) or phase 4 (verify) — those are
where wikis fail: undocumented assumptions and unverified claims.

### Phase 1 — Discover

Map the repository's real shape before reading any file in depth. The goal is
a skeleton you will flesh out in Phase 2.

1. **Read the manifests first.** Read `go.mod`, `Makefile`, `README.md`,
   `AGENTS.md`, `CONTRIBUTING.md`, and any `docs/` index. These establish the
   module path, Go version, declared dependencies, build targets, and stated
   conventions. Record the module path and Go version — every package path in
   the wiki derives from the module path in `go.mod`.

2. **Map the directory tree.** Walk the top 3 levels of the repository. Record
   the directory structure as a tree. Identify the command entrypoints
   (`cmd/`), internal packages (`internal/`), public packages (`pkg/` if
   present), test directories, and documentation. Skip `vendor/`, `bin/`,
   `dist/`, `.git/`, and generated output directories.

3. **Identify entrypoints.** Find every `package main` and `func main()`.
   These are the executable targets. For a CLI/TUI like pcb, entrypoints live
   under `cmd/`. Record each entrypoint's path, its purpose (from the package
   doc comment or README), and what it imports from `internal/`.

4. **Detect the architecture pattern.** Determine the layering direction.
   For Go CLIs/TUIs, the common pattern is `cmd → app → feature → core/
   infrastructure`. Read any `docs/design/principles.md` or
   `docs/reference/dependency-rules.md` if present — these state the intended
   direction. Record it. Every cross-reference in the wiki must respect this
   direction; a wiki that documents a violation without flagging it is
   complicit in the violation.

5. **Build the package map.** For each package directory under `cmd/`,
   `internal/`, and `pkg/`, record:
   - The import path (module path + relative dir).
   - A one-line responsibility (read the package doc comment; if absent, read
     the top-level types and functions to infer, and mark **(inferred)**).
   - Whether it has tests (`*_test.go` present).
   - Its direct imports (from the `import` block — read it, do not guess).

6. **Record a knowledge summary.** Write a brief internal note (not shipped)
   capturing: module path, Go version, architecture direction, entrypoints,
   package count, and any anomalies (cycles, TODOs in package docs, packages
   with no tests). Distinguish "verified by reading" from "inferred."

### Phase 2 — Analyze

Deep-read the packages that matter. Not every package needs equal depth —
prioritize by fan-in (packages imported by many others) and by entrypoint
proximity.

1. **Analyze entrypoints first.** For each `cmd/` entrypoint, read `main.go`
   and trace the call chain 2-3 levels deep. Record: what flags/args are
   parsed, what app-layer function is called, and the initialization sequence.
   This establishes the runtime entry into the architecture.

2. **Analyze core packages.** Read the packages in the dependency-light core
   (`internal/core/` or equivalent). For each:
   - List exported types with their fields and embedded types.
   - List exported interfaces with their method signatures.
   - List exported functions with their signatures.
   - Record the package's responsibility in 2-3 sentences grounded in the
     actual code, not the name.
   - Note key unexported types only if they are load-bearing (e.g., a struct
     that implements an exported interface).

3. **Analyze feature packages.** Read packages under `internal/app/`,
   `internal/feature/`, or domain-specific directories. For each, record:
   - What domain capability it provides.
   - Which core types it depends on.
   - Which interfaces it implements or consumes.
   - Any state it owns (structs with mutable fields, registries, caches).

4. **Analyze infrastructure packages.** Read packages handling I/O, config,
   persistence, or external integrations (`internal/infrastructure/`,
   `internal/confdir/`, `internal/atomicfile/`, etc.). Record the abstraction
   they provide and what concrete system they wrap.

5. **Trace the dependency graph.** From the import blocks you read, construct
   a directed graph of package dependencies. Verify it respects the
   architecture direction. If a cycle or a reverse-direction dependency
   exists, record it explicitly — the wiki must document reality, including
   violations of stated rules.

6. **Identify key data flows.** For the primary user journey (e.g., launching
   the TUI, running a CLI command, loading a skill), trace the data path
   through packages: input → parse → dispatch → core → output. Record the
   package sequence and the types that cross each boundary.

7. **Read the tests as documentation.** Test files reveal intended behavior
   and non-obvious invariants. For packages with tests, note what behaviors
   are asserted and any table-driven cases that illustrate edge cases. Do not
   copy test code into the wiki — summarize the guarantees.

### Phase 3 — Document

Write the four wiki files. Generate in this order so each file can reference
the previous:

#### File 1: `ARCHITECTURE.md`

The system-level view. A reader should understand the codebase shape and
layering from this file alone.

Required sections:

- **Overview** — 2-3 paragraphs: what the project is, its primary runtime
  mode (CLI, TUI, server), and the problem it solves. Grounded in README and
  the actual entrypoints.
- **Module & runtime** — module path, Go version, build targets (from
  `Makefile` or `cmd/`), and how to build/run (reference the Makefile target
  names, do not invent commands).
- **Layering** — the architecture direction (e.g., `cmd → app → feature →
  core/infrastructure`). Include a Mermaid `graph LR` diagram showing the
  layers and key packages. Keep the diagram readable — group by layer, not
  by alphabet.
- **Entrypoints** — each `cmd/` target, its path, purpose, and what it
  delegates to. Include `file:line` references to `main.go`.
- **Key data flows** — 2-3 primary journeys traced through packages, with the
  package sequence and the types crossing boundaries. Use numbered steps.
- **Cross-cutting concerns** — logging, error handling, configuration, theme/
  rendering (for TUIs). Name the package responsible for each.
- **Dependency direction rules** — state the intended direction and note any
  observed violations with `file:line` references.

#### File 2: `PACKAGES.md`

The package catalog. Every package gets an entry.

Required structure per package:

- **Import path** — full path from `go.mod`.
- **Responsibility** — 2-3 sentences grounded in actual code.
- **Key exports** — exported types, interfaces, and functions, each with a
  one-line description. Group by kind (Types, Interfaces, Functions).
- **Dependencies** — direct imports, categorized as internal (same module) or
  external (third-party).
- **Dependents** — packages that import this one (derived from the dependency
  graph; list only if non-empty).
- **Tests** — whether tests exist and what they broadly assert.
- **Location** — `file:line` reference to the package's primary file.

Order packages by layer (cmd, then app/feature, then core, then
infrastructure), and alphabetically within a layer. This mirrors the
architecture direction and makes navigation intuitive.

#### File 3: `API.md`

The exported API reference. Covers types, interfaces, and exported functions
across all public and internal packages.

Required structure:

- **Types** — for each exported struct: name, fields (name, type, tags),
  embedded types, and a one-line purpose. Use the actual field names and types
  from the source — never paraphrase types.
- **Interfaces** — for each exported interface: name, full method set with
  signatures, and which packages provide implementations. If an interface is
  satisfied by multiple types, list each implementer.
- **Functions** — for each exported function: signature, parameters, return
  values, and a one-line description of behavior. Note side effects (writes
  to disk, mutates shared state, spawns goroutines) when observable.
- **Constants & variables** — exported constants and package-level variables
  that are part of the API surface.
- **Constructors** — exported `New*` functions, what they return, and what
  dependencies they wire together.

Group entries by package, with a package-level heading and a back-link to the
package's entry in `PACKAGES.md`. Within a package, order: Constants,
Variables, Types, Interfaces, Functions (constructors first).

#### File 4: `WORKFLOWS.md`

The behavioral view. Documents how the system operates at runtime.

Required sections:

- **Startup sequence** — from process launch to ready state. Numbered steps
  with package/function references. For a TUI: model initialization, view
  setup, event loop entry. For a CLI: arg parsing, config load, dispatch.
- **Primary command flows** — 2-3 key user actions traced step-by-step through
  the code. For each: the entry point, the packages traversed, the types
  transformed, and the output/side effect. Use `file:line` references.
- **Event loop / dispatch** — for a TUI, how messages flow through the
  Model/Update/View cycle and which packages handle each message type. For a
  CLI, how commands dispatch to handlers.
- **State lifecycle** — how key stateful objects (registries, caches,
  sessions) are created, mutated, and torn down.
- **Error paths** — how errors propagate from core to the user-facing layer.
  Name the error types and the presentation package.
- **Build & release workflow** — Makefile targets, versioning scheme (from
  `CHANGELOG.md` or version constants), and any CI-relevant targets. State
  only what is evidenced in the Makefile or scripts; do not invent CI
  pipelines you did not read.

### Phase 4 — Verify

The wiki is not done until it is verified against the code. A wiki with a
single hallucinated type signature is a liability — every downstream reader
inherits the error.

1. **Cross-check exports.** For every type, interface, and function listed in
   `API.md`, confirm the name, signature, and field list against the source.
   Re-read the file if there is any doubt. Fix the wiki, not the code.

2. **Cross-check dependencies.** For every "Dependencies" entry in
   `PACKAGES.md`, confirm the import list matches the `import` block in the
   package's files. Remove any import you did not observe.

3. **Cross-check the dependency direction.** Walk the dependency graph and
   confirm the direction stated in `ARCHITECTURE.md` matches the actual
   imports. If a violation was documented, confirm it still exists (the code
   may have been fixed since you read it).

4. **Cross-check data flows.** For each workflow in `WORKFLOWS.md`, re-trace
   the call chain against the source. Confirm every `file:line` reference
   resolves to the function claimed.

5. **Check links.** Every relative link between wiki files must resolve.
   Every `file:line` reference must point to a real location. Use the
   repository's path style consistently (e.g., `internal/skill/loader.go:225`).

6. **Check for placeholders.** Scan all four files for "TODO", "<fill here>",
   "TBD", or empty sections. Either fill them with verified content or mark
   them `N/A — <reason>`. A wiki with placeholder text is worse than no wiki.

7. **Check the 100% claim.** If `ARCHITECTURE.md` claims the layering is
   `cmd → app → feature → core/infrastructure`, verify no package violates it
   unflagged. If a violation exists and is unflagged, either flag it or
   correct the claim.

8. **Summary.** After verification, print a brief summary: the output path,
   one line per file (its section count and package coverage), and the count
   of `file:line` references. Note any packages or flows that were marked
   **(inferred)** rather than verified by reading.

---

This is a Go project. Go-specific documentation conventions apply throughout.

### Types

- Document exported structs with their full field list: field name, type, and
  struct tag (especially `json:`, `yaml:`, `db:` tags — these define the
  serialization contract).
- Document embedded types explicitly — embedding is Go's composition mechanism
  and determines the promoted method set. List embedded types before declared
  fields.
- For unexported types that are load-bearing (implement an exported interface,
  or are returned by an exported function wrapped in an interface), document
  them under the package's "Internal types" note with a clear reason for
  inclusion.

### Interfaces

- Document the full method set. Every method signature must match the source
  exactly — parameter names, types, and return values.
- List implementers. Go interfaces are implicit; to find implementers, search
  for methods matching the signature. Record which packages contain types
  that satisfy the interface, and mark the match as **(verified)** if you read
  the implementing type, or **(inferred)** if you matched by signature alone.
- Note if the interface is a single-method interface (common in Go: `error`,
  `io.Reader`, handler types) — these are often used as function adapters.

### Exported functions

- Document the full signature: `func FuncName(params) (returns)`.
- Note constructors (`New*`) separately and list what they wire together —
  constructors reveal the dependency graph.
- For functions that return interfaces, note the concrete return type if it
  is observable (e.g., returns `*ConcreteType` satisfying `Interface`).
- Side effects: if a function writes to disk, mutates a package-level
  variable, spawns a goroutine, or acquires a lock, document it. These are
  the behaviors that surprise readers.

### Package relationships

- Go packages are the primary unit of modularity. The wiki's package map is
  the backbone — every other file references it.
- Document the import graph direction. In a well-structured Go project,
  `internal/` packages are not importable outside the module; note this as a
  scoping boundary.
- For each package, note whether it is in `cmd/` (entrypoint), `internal/`
  (private), or `pkg/` (public, importable by other modules). If `pkg/` does
  not exist, state that all non-cmd packages are internal.
- Document test-only packages (`*_test.go` or `tests/` directories)
  separately — they are not part of the runtime API surface but document
  intended behavior.

### Go-specific conventions for this wiki

- Use the full import path as the package identifier (e.g.,
  `github.com/boytegar/packboy-builder/internal/skill`), derived from
  `go.mod`. Never abbreviate or invent a path.
- When referencing a symbol, use `package.Symbol` form (e.g.,
  `skill.Registry`).
- When referencing a file location, use the repository-relative path with a
  line number (e.g., `internal/skill/loader.go:225`).
- Document the Go version from `go.mod` in `ARCHITECTURE.md`. If the project
  uses generics, note the minimum Go version that supports the generics used.
- If the project uses `embed` (as pcb does for builtin skills), document the
  embedded directory and the materialization function — this is a
  non-obvious build-time mechanism.

---

Consistent formatting makes the wiki scannable and trustworthy.

### Headings

- `#` for the file title only (one per file).
- `##` for major sections (e.g., `## Entrypoints`).
- `###` for subsections (e.g., `### cmd/pcb`).
- `####` for per-package or per-type entries within a section.
- Never skip a level. `#` → `##` → `###` → `####`. No `#` → `###`.

### Code references

- Inline code: `` `package.Symbol` `` for symbols, `` `file.go:225` `` for
  locations.
- Fenced code blocks for multi-line signatures, directory trees, or
  configuration. Always specify the language: `go`, `bash`, `text`, `mermaid`.
- For directory trees, use a fenced `text` block with the standard tree
  characters (`├──`, `└──`, `│`).

### Mermaid diagrams

- Use `mermaid` fenced blocks. `graph LR` for architecture and dependency
  direction; `flowchart TD` for workflows; `erDiagram` is not needed
  (Go has no relational schema, but type relationships can use `classDiagram`
  if useful).
- Keep diagrams to 10-15 nodes. For larger graphs, split into subgraphs by
  layer. A diagram with 30 nodes is unreadable.
- Every node in a diagram must correspond to a package or type documented
  elsewhere in the wiki. No orphan nodes.

### Tables

- Use Markdown tables for structured comparisons (e.g., package responsibility
  summaries, type field lists).
- Column headers in sentence case. Keep tables to 4-5 columns — wider tables
  render poorly in wiki viewers.

### Cross-references

- Link between wiki files with relative links: `[PACKAGES.md](PACKAGES.md)`.
- Link to a section: `[skill package](PACKAGES.md#skill)`. Lowercase the
  heading, replace spaces with hyphens.
- Link to source files with the repository-relative path (not a wiki-internal
  link): `` `internal/skill/loader.go` `` in inline code. The wiki viewer
  resolves these to the source.

### Tone and voice

- Present tense, active voice: "The loader reads SKILL.md files," not "Files
  are read by the loader."
- Declarative, not hedging. If you verified it, state it. If you inferred it,
  mark **(inferred)** and state the basis.
- No marketing language. The wiki is a reference, not a pitch.

### Length and depth

- `ARCHITECTURE.md`: 150-300 lines. Enough for a system view, not so much that
  it becomes a package catalog (that is `PACKAGES.md`'s job).
- `PACKAGES.md`: one entry per package, 15-40 lines per entry depending on
  complexity. Total scales with package count.
- `API.md`: one entry per exported symbol, 5-15 lines per entry. Total scales
  with API surface.
- `WORKFLOWS.md`: 100-250 lines. 2-3 workflows traced in depth beat 10
  workflows traced shallowly.

---

Linking is what makes a wiki navigable rather than a stack of disconnected
documents. Every package, type, and flow should be reachable from every other
relevant entry.

### Package-to-package links

- In `PACKAGES.md`, the "Dependencies" list links to the dependency's package
  entry: `[skill](#skill)` for same-file links, or `[skill.Registry](API.md#skillregistry)`
  for cross-file links to a specific symbol.
- The "Dependents" list is the reverse: which packages import this one. This
  is derived from the dependency graph in Phase 2. If a package has no
  dependents (it is a leaf or a root entrypoint), state "None (leaf package)"
  or "None (root entrypoint, imported only by `cmd/`)" explicitly.

### Type-to-interface links

- In `API.md`, under an interface, link to each implementer's type entry:
  `Implemented by: [Registry](#registry), [Store](#store)`.
- Under a struct, if it implements an interface, link to the interface:
  `Implements: [StateChangeObserver](#statechangeobserver)`.
- These bidirectional links let a reader navigate from an interface to its
  implementations and back.

### Flow-to-source links

- In `WORKFLOWS.md`, every step in a workflow references the source location:
  `1. Parse flags ([cmd/pcb/main.go:42](\`cmd/pcb/main.go:42\`))`.
- The reference is both a navigation aid and a verification anchor — the
  reader (and the verifier) can confirm the step by reading the cited line.

### Data-flow tracing method

To trace a data flow for `WORKFLOWS.md`:

1. Start at the entrypoint (e.g., `func main()` in `cmd/pcb/main.go`).
2. Read the first call into the app layer. Record the function and the types
   of its arguments — these are the data crossing the boundary.
3. Follow the call chain, recording each package boundary crossed and the
   type transformation at each boundary.
4. End at the output: a rendered TUI view, a written file, a printed result.
   Record the output type and the package that produces it.
5. Format as a numbered list with `file:line` references at each step. Add a
   one-line summary of the data type at each boundary.

This method produces a trace a reader can follow by reading the cited lines in
order — the wiki becomes a guided tour of the codebase, not a static map.

---

Write exactly these four files into the output directory (default: `wiki/` at
the repository root; create the directory if missing; do not overwrite
existing wiki files without confirming in Phase 1):

```
wiki/
├── ARCHITECTURE.md
├── PACKAGES.md
├── API.md
└── WORKFLOWS.md
```

Each file follows the structure specified in Phase 3. The files cross-reference
each other: `ARCHITECTURE.md` links to package entries in `PACKAGES.md`;
`PACKAGES.md` links to symbol entries in `API.md`; `WORKFLOWS.md` links to all
three. A reader entering at any file can reach the others.

### Output order

Generate in this order so each file can reference the previous:

1. `PACKAGES.md` — the package catalog is the backbone; architecture and API
   both reference it.
2. `API.md` — the symbol reference builds on the package entries.
3. `ARCHITECTURE.md` — the system view references packages and key types.
4. `WORKFLOWS.md` — the behavioral view references all of the above.

This order differs from the reading order (a reader starts with
`ARCHITECTURE.md`) but is correct for generation: you build the catalog and
symbol reference first, then synthesize the system and behavioral views from
them.

### Existing wiki files

If `wiki/` already contains any of the four files, read them before
generating. The user may be asking to refresh, not start fresh. Note which
exist and what they cover. If the existing content is accurate and current,
merge rather than overwrite — preserve verified `file:line` references and
human-written prose. If stale, regenerate from scratch.

---

- Never write or modify source files. The only files you create are the four
  under `wiki/`.
- Never run builds, tests, or long-running commands. Documentation only. You
  may run read-only inspection commands (grep, find, go list) but not builds.
- Never invent types, functions, fields, or dependencies. Every exported
  symbol in `API.md` must be read from source. Every import in `PACKAGES.md`
  must be read from the `import` block.
- Never present an inferred claim as a verified one. Mark inferences with
  **(inferred)** and state the basis.
- Never copy source code into the wiki. Summarize signatures and behavior;
  do not paste function bodies. The wiki points to the source, it does not
  duplicate it.
- Never document a package you did not read. If a package is inaccessible or
  too large to read in the current pass, omit it and note the gap in the
  Phase 4 summary — do not fabricate an entry from the package name.
- Never skip Phase 4 (verify). A wiki that is not verified is a draft, not a
  deliverable. If time or context is constrained, narrow the scope (fewer
  packages, fewer workflows) rather than shipping unverified content.
- If `wiki/` already contains a file you are about to regenerate, confirm
  with the user (overwrite / merge / skip) before touching it.
- All output is in English regardless of the language used in code comments
  or the requesting prompt. Translate intent, not meaning.

---

### Standard Go inspection commands

For Go-specific discovery, these read-only commands are safe and useful:

```bash
go list ./...                         # all package import paths
go list -deps ./cmd/...               # dependency tree per entrypoint
go doc <pkg>                          # package documentation
go doc -all <pkg>                     # full package documentation
go doc <pkg>.<Type>                   # type documentation
go doc <pkg>.<Func>                   # function signature + doc
```

Use `go doc` to verify signatures during Phase 4 — it reads the compiled
package and is authoritative. If `go doc` output disagrees with your wiki,
fix the wiki.

### Subagent delegation

For repositories with more than 15-20 packages, dispatch `researcher`
subagents (mode=explore) to analyze package clusters in parallel. Assign
each subagent a layer (cmd, app/feature, core, infrastructure) and have it
return the package map entries (responsibility, exports, dependencies) for
that layer. Consolidate the results into `PACKAGES.md` and `API.md` yourself
to ensure consistent formatting and cross-references. Do not delegate the
writing of `ARCHITECTURE.md` or `WORKFLOWS.md` — those require a holistic
view that no single subagent has.

### Context budget

If context is constrained, narrow scope rather than shipping incomplete
content:

1. Cover all packages in `PACKAGES.md` (even briefly) — an incomplete catalog
   is a navigation dead end.
2. Cover all exported types and interfaces in `API.md` — exported functions
   can be summarized more briefly if needed.
3. Limit `WORKFLOWS.md` to 1-2 primary flows rather than 3.
4. Never narrow `ARCHITECTURE.md` — it is the entry point for readers and
   must be complete.

State any scope reductions in the Phase 4 summary so the reader knows what is
not covered.
