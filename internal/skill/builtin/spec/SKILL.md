---
name: spec
description: Generate project context files (PRD.md, ARCHITECTURE.md, DESIGN.md, SCHEMA.md, RULES.md) in context/ after a clarification pass to reach 100% understanding.
argument-hint: "[product/feature description]"
---

# Spec Generation

`/spec` produces a complete `context/` directory of AI-agnostic specification
files so that any coding agent (Cursor, Windsurf, Claude, Copilot) dropped into
the repo produces code consistent with the project's intent, architecture,
design system, data model, and coding rules.

The five output files are not invented from thin air — they are derived from
what the user wants AND what the codebase already contains. A spec for a
greenfield project differs from a spec layered onto an existing codebase. Detect
which case you are in before generating.

## Output

Write exactly these five files into `context/` (create the directory if
missing; do not overwrite existing files without confirming in Phase 1):

```
context/
├── PRD.md
├── ARCHITECTURE.md
├── DESIGN.md
├── SCHEMA.md
└── RULES.md
```

Each file follows its reference template. Load the template lazily — read the
matching `references/*.md` only when you are about to write that file, to keep
your context lean.

- [references/prd.md](references/prd.md) — PRD template & required sections.
- [references/architecture.md](references/architecture.md) — architecture template (Mermaid diagram + tech stack).
- [references/design.md](references/design.md) — UI/UX & component template.
- [references/schema.md](references/schema.md) — database schema & ERD template.
- [references/rules.md](references/rules.md) — coding standards & Do/Don't template.

## Phase 1 — Understand the request (target: 100%)

You are a SPEC WRITER, not an implementer. You do not write source code, run
builds, or apply edits to source files. The only files you create are the five
files under `context/`.

The user's argument string after `/spec` is the product/feature description.
Treat it as {{prompt}}. If empty, treat {{prompt}} as empty and follow the
empty-prompt guardrail below.

### 1a. Ground yourself

Before assessing understanding, inspect the current repository:

- Read `README.md`, `AGENTS.md`, `CLAUDE.md`, `package.json`/`go.mod`/`Cargo.toml`
  /`pyproject.toml`/`pom.xml` (whichever exists) to learn the tech stack and
  conventions already in use.
- Scan the directory tree (top 2 levels) to learn the project shape.
- If `context/` already exists with any of the five files, read them — the user
  may be asking to refresh or extend, not start fresh. Note which exist.
- If the repo is empty or near-empty, this is a greenfield spec — derive
  everything from {{prompt}}.
- Spawn a `researcher` subagent (mode=explore) when grounding touches more than
  1-2 files, to keep your own context lean.

Record a brief knowledge summary with `file:line` references. Distinguish
"verified by reading" from "inferred".

### 1b. Self-assess understanding (target: 100%)

Score your understanding 0-100% against these dimensions. The target is 100%,
not 90% — a spec written on partial understanding becomes a load-bearing
hallucination that every downstream agent inherits.

- **Product vision**: the problem, the value proposition, the primary user
  journey. Can you state in one sentence what success looks like?
- **Scope boundary**: what is IN this spec and what is explicitly OUT. If you
  cannot name the out-of-scope items, you do not understand the scope.
- **Tech stack**: languages, frameworks, database, hosting. For an existing
  repo these are read from the manifest files; for greenfield they must be
  decided and confirmed.
- **Data model shape**: the core entities and their cardinalities. You do not
  need every field, but you need the main entities and how they relate.
- **User roles & permissions**: who uses it and what they can do. If the
  product has auth, name the roles. If it has no auth, state that explicitly.
- **Non-functional targets**: scale, performance, compliance constraints. If
  none are stated, confirm the default (small-scale, no specific SLA) rather
  than inventing SLAs.

Compute the score honestly. `= 100%` → go to Phase 2.
`< 100%` → do NOT generate yet. Call the `AskUserQuestion` tool with targeted
questions that close the SPECIFIC gaps you found. Rules for the questions:

- Each question must be one the user can actually answer (not "tell me
  everything about your product").
- 2-8 options each, grounded in the concrete choices the product presents
  (e.g. "PostgreSQL" vs "MongoDB", not "what database do you want").
- Prefer options that include a sensible default marked with "(recommended)" so
  the user can pick fast.
- Do NOT ask questions answerable by reading more of the repo — go read instead.
- Do NOT ask about visual style preferences in Phase 1 — those belong to
  DESIGN.md and can use sensible defaults the user overrides later.

After the user answers, return to 1a to verify their answers against the repo
(users can be wrong), then re-score. Cap the loop at 3 question rounds. If
still `< 100%` after 3 rounds, proceed to Phase 2 and write the files, but
prefix each file with an `> ⚠ Unresolved assumptions` callout listing every
open gap — do not stall forever and do not fill gaps with guesses presented as
fact. Guesses labeled as guesses (in the callout) are acceptable; guesses
presented as requirements are the failure mode this command exists to prevent.

### Empty-prompt guardrail

If {{prompt}} is empty, the first thing you do is call `AskUserQuestion` with
one question: "What product or feature do you want to spec?" and a few example
options (e.g. "A new web app", "An API service", "A CLI tool", "A mobile
app"). Use the answer as {{prompt}} and continue Phase 1.

## Phase 2 — Generate the five files

Write all five files to `context/`. For each file:

1. Read the matching `references/<file>.md` template.
2. Fill every section. Do not leave placeholder text like "TODO" or "<fill
   here>" — if a section genuinely does not apply, write "N/A — <reason>"
   rather than deleting it. A consistent section structure across projects is
   what makes the context files useful to downstream agents.
3. Use Mermaid.js (`\`\`\`mermaid` fenced blocks) for the system architecture
   diagram in ARCHITECTURE.md and the ERD in SCHEMA.md. Keep diagrams
   readable — prefer `graph LR` for architecture and `erDiagram` for the ERD.
4. Ground every claim. For an existing repo, tech stack and directory structure
   must match what you read. For greenfield, the stack must match what was
   confirmed in Phase 1 — if the user picked PostgreSQL, SCHEMA.md uses
   PostgreSQL types, not generic "VARCHAR".
5. Cross-reference between files: ARCHITECTURE.md's data layer must point at
   SCHEMA.md's entities; DESIGN.md's state management must align with
   ARCHITECTURE.md's frontend stack; RULES.md's conventions must match the
   language actually chosen. Inconsistency between the five files is the
   second failure mode this command exists to prevent.

### Generation order (matters)

Generate in this order so each file can reference the previous:

1. `PRD.md` — establishes scope, users, requirements. Everything else derives
   from here.
2. `SCHEMA.md` — the data model is the backbone; architecture and design both
   depend on it.
3. `ARCHITECTURE.md` — tech stack and service boundaries must serve the PRD's
   functional requirements and sit on top of the schema.
4. `DESIGN.md` — UI flows map to the architecture's frontend layer and expose
   the schema's entities to the user.
5. `RULES.md` — coding standards are scoped to the chosen tech stack, so they
   go last.

## Phase 3 — Report

After writing all five files, print a brief summary in chat:

- The `context/` path.
- One line per file: its primary focus (e.g. "PRD.md — 4 user roles, 12
  functional requirements").
- The final understanding score.
- Count of clarifying questions asked across Phase 1.
- If any file has an `⚠ Unresolved assumptions` callout, name the file and the
  open gaps.

Do not paste the file contents into chat — the files are the artifacts.

## Guardrails

- Never write or modify source files. The only files you create are the five
  under `context/`.
- Never run builds, tests, or long-running commands — spec writing only.
- Never invent requirements, entities, or tech choices and present them as
  confirmed. Unconfirmed items go in the `⚠ Unresolved assumptions` callout.
- Never copy a reference template verbatim into `context/` — templates are
  skeletons; the output must be filled with the project's actual content.
- If `context/` already contains a file you are about to write, confirm with
  the user (one `AskUserQuestion`: overwrite / merge / skip) before touching it.
- All output is in English regardless of the language the user used in
  {{prompt}}. If the user wrote in another language, translate their intent
  into English for the generated files but do not change the meaning.
