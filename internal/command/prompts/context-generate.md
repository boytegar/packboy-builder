# SKILL: PROJECT_CONTEXT_ARCHITECT
**Version:** 3.1
**Objective:** Analyze a real repository and generate a repository-specific AI guidance pack consisting of a top-level `AGENTS.md` and a `.agents/` documentation set that matches the actual language, framework, tooling, structure, and workflows detected in the codebase.

---

## I. CORE MISSION

Your job is not to create JavaScript-centric boilerplate.

Your job is to reverse-engineer the repository's real engineering rules from repository evidence, then generate an AI guidance pack that is:
- specific to the actual repository
- aligned to the detected language(s), framework(s), runtime(s), and tools
- practical enough for an AI agent to modify the repository safely
- detailed enough to reduce ambiguity for future implementation work
- honest about uncertainty when the repository does not prove a rule

The generated output must work for repositories built with any real stack present in the codebase, including but not limited to:
- JavaScript / TypeScript
- Python
- Go
- Rust
- PHP
- Java / Kotlin
- Ruby
- C#
- Dart / Flutter
- mixed-language or multi-package repositories

If the repository is incomplete or ambiguous, prefer:
- `Not detected in repository`
- `Needs verification`
- `Inferred from current structure`

Never invent certainty when the repository does not support it.

---

## II. MANDATORY ANALYSIS PHASE

Before writing any documentation, inspect the repository thoroughly.

### 1. Language, Runtime, and Dependency Detection
Check all applicable manifests, lockfiles, version files, and language-specific tooling files, including but not limited to:
- `package.json`
- `pnpm-lock.yaml`
- `package-lock.json`
- `yarn.lock`
- `bun.lock*`
- `tsconfig.json`
- `jsconfig.json`
- `deno.json`
- `go.mod`
- `go.sum`
- `Cargo.toml`
- `Cargo.lock`
- `requirements.txt`
- `pyproject.toml`
- `poetry.lock`
- `Pipfile`
- `uv.lock`
- `pubspec.yaml`
- `Gemfile`
- `Gemfile.lock`
- `composer.json`
- `composer.lock`
- `pom.xml`
- `build.gradle`
- `build.gradle.kts`
- `settings.gradle`
- `settings.gradle.kts`
- `libs.versions.toml`
- `.nvmrc`
- `.node-version`
- `.python-version`
- `.ruby-version`
- `.tool-versions`
- `Dockerfile`
- `docker-compose.yml`
- `docker-compose.yaml`
- `Makefile`

Extract and record:
- primary language(s)
- framework(s)
- package manager(s)
- runtime(s) and version constraints
- build tools
- testing tools
- linting and formatting tools
- type-checking tools
- code generation tools
- migration tools
- deployment or infrastructure tooling if present

### 2. Project Structure Mapping
Map the repository structure up to depth 3 or 4.

Identify:
- entry points
- source roots
- package or app boundaries
- feature folders
- shared/common layers
- config directories
- infrastructure files
- test directories
- scripts and automation folders
- generated-code locations
- schema, migration, or contract directories

Infer likely architecture using repository evidence:
- feature-based
- layer-based
- MVC
- MVVM
- clean architecture
- modular monolith
- microservices
- monorepo / workspace
- package-based workspace
- hybrid structure

### 3. Representative Code Inspection
Inspect representative source files, not just manifests.

At minimum, inspect:
- 2 files with business logic, services, controllers, handlers, commands, or use cases
- 2 files with models, schemas, DTOs, serializers, entities, types, or state definitions
- 2 files that serve as UI entry points, routes, controllers, API handlers, CLIs, workers, or adapters
- 1 important configuration file
- 1 test file if tests exist

Detect from real code:
- naming style
- file naming style
- folder naming style
- function and method patterns
- component, controller, or handler structure
- type strictness
- state management style
- dependency injection or composition patterns
- error handling style
- import and export conventions
- file organization habits
- stack-specific implementation patterns

### 4. Development Workflow Detection
Inspect project tooling and automation.

Check:
- package scripts
- Makefiles
- task runners
- Dockerfiles / docker-compose
- CI/CD workflows
- `.env.example`
- `.github/workflows`
- pre-commit / lint-staged / hooks if present
- migration / seed scripts if present
- codegen scripts if present

Extract real or strongly evidenced commands for:
- install
- dev
- build
- lint
- format
- typecheck
- unit test
- integration test
- e2e test
- code generation
- migration / seed
- run / serve / start

If a command is not clearly defined, say so explicitly.

### 5. Agent-Relevant Constraints
Identify anything that affects safe autonomous edits, such as:
- generated files
- codegen workflows
- strict lint or type gates
- path aliases or module path rules
- monorepo or workspace boundaries
- required environment variables
- deployment-sensitive directories
- schema, migration, or contract-sensitive files
- files that should not be edited manually
- framework-specific boundaries
- language-specific constraints

---

## III. EVIDENCE EXTRACTION RULES

### Non-Negotiable Rules
- Do not recommend tools that are not found in the repository unless clearly labeled as optional suggestions outside enforced project rules.
- Do not assume the stack is JavaScript-based.
- Do not force JavaScript, React, Node.js, or frontend terminology onto repositories that use other ecosystems.
- Do not claim an architectural pattern without evidence from folder layout, config, and source files.
- Do not fabricate commands.
- Do not assume test coverage exists if no tests are present.
- Do not assume linting, formatting, typecheck, CI, or deployment workflows exist unless detected.
- Mirror the repository's actual naming and structural conventions.
- Use terminology appropriate to the detected stack. Examples:
  - `controller`, `service`, `entity` for backend MVC-style systems
  - `component`, `screen`, `hook` for React-like UI systems
  - `handler`, `router`, `middleware` for web backends
  - `command`, `subcommand`, `package` for CLI tools
  - `crate`, `module`, `trait` for Rust
  - `package`, `struct`, `interface` for Go
  - `model`, `serializer`, `view`, `app` for Django-like systems

### Evidence Language
Every major section should be based on repository evidence such as:
- manifest files
- lockfiles
- config files
- source files
- test files
- CI definitions
- README or setup docs

Use phrasing like:
- `Detected from package.json and src/...`
- `Detected from go.mod and cmd/...`
- `Detected from pyproject.toml and app/...`
- `Inferred from folder structure and handler layout`
- `Observed in controller and service implementations`
- `Observed in components and route definitions`
- `Not clearly defined in repository`
- `Needs verification before enforcement`

If evidence is weak, state uncertainty plainly.

### Anti-Hallucination Policy
If the repository does not prove a rule, do not present it as a repository rule.

Allowed fallback language:
- `No dedicated formatter detected`
- `No test runner explicitly configured`
- `Architecture appears feature-based, but boundaries are partially mixed`
- `Command not found in manifest; verify manually`
- `Framework not conclusively detected from current repository state`

---

## IV. STRICT OUTPUT BEHAVIOR

- Do not give recommendations, suggestions, opinions, or best-practice advice.
- Do not add improvement ideas, optional enhancements, or modernization proposals.
- Do not use language such as `should`, `consider`, `you may`, `optionally`, `recommended`, or `best practice` unless quoting repository text.
- Only document what is directly detected, strongly evidenced, or explicitly marked as uncertain.
- If evidence is missing, state the gap and stop there.
- The goal is repository context extraction, not repository consultation.
- Default to descriptive documentation, not prescriptive guidance.
- Do not add stack recommendations beyond what the repository already uses.

### Skill Format Compatibility
- This skill must remain usable as a plain Markdown skill file.
- Do not require YAML-only skill metadata or YAML frontmatter to make the skill usable.
- If a toolchain supports YAML skills, this file may be adapted later, but the canonical source must stay fully functional as Markdown.
- Write instructions so they can be executed correctly even in environments where YAML-defined skills are not supported.

---

## V. OUTPUT REQUIREMENTS

Generate:
- top-level `AGENTS.md`
- `.agents/overview.md`
- `.agents/style.md`
- `.agents/design.md`
- `.agents/components.md`
- `.agents/routes.md`
- `.agents/memory/` with today's file named `date-month-year-memory.md`
- conditional: `.agents/skills/` (only if user confirms skills implementation)

The `.agents/` files are semantic buckets. Their content must adapt to the repository type.
Do not force UI-specific or route-specific assumptions into non-UI repositories.
If a file category is only partially applicable, keep the file but rename the section labels inside it to fit the detected stack.

Examples:
- In a backend API repo, `routes.md` may document HTTP routes, handlers, controllers, jobs, or transport boundaries.
- In a CLI repo, `routes.md` may instead document command entry points, subcommands, and execution flow.
- In a library repo, `components.md` may document modules, packages, adapters, public APIs, and reusable building blocks.
- In a mobile or frontend app, `components.md` may document components, layouts, screens, hooks, and shared UI primitives.

### 1. `AGENTS.md`
Purpose: concise entry point for AI agents.

Must include:
- role and objective
- explicit note that skills generation is conditional based on user confirmation before generate starts
- instruction to consult the `.agents/` directory
- needs-based reading guidance for `.agents/` documentation (not mandatory full read):
  - agents must read only the files relevant to the current task scope
  - start with `./.agents/overview.md` for repository orientation when task context is unclear
  - read `./.agents/style.md` when implementing or modifying code conventions
  - read `./.agents/design.md` only for visual/UI design work
  - read `./.agents/components.md` when touching reusable building blocks or placement decisions
  - read `./.agents/routes.md` when touching routes, handlers, command flow, navigation, or execution boundaries
  - read `./.agents/skills/` only when user confirmed skills implementation and the task requires those artifacts
  - read `./.agents/memory/<date-month-year-memory.md>` when continuity from today's prior work is required
  - if a task spans multiple concerns, read the corresponding combination of relevant files
- explicit session-scope rule: `.agents/` documentation is read once at the start of a session; if already read in the same session, do not re-read unless the `.agents/` files changed or task scope shifts to previously unread relevant sections
- concise operating rules
|- explicit scaling rule: when task scope is large or contains many parallelizable work items, agents must use a swarm-agent approach and set the total number of agents as optimally as possible for the task complexity and dependency graph
|- explicit rule that agents must use subagents/agent swarm for exploration whenever the task includes repository discovery, searching, or non-trivial investigation
|- explicit rule that exploration work must be delegated to subagents/agent swarm before a single-agent path is used, unless the task is trivially small and fully local
|- explicit rule that agents must use test-driven development (TDD) whenever adding, editing, or deleting code to reduce regression risk and avoid avoidable errors
- explicit pre-execution rule that agents must read `./.agents/skills/` first so each execution follows available skills artifacts
- explicit rule that agents must not run `git push` or any remote push action
- explicit rule that agents must not work outside the repository/directory scope documented by `AGENTS.md`
- a dedicated textfield-style prompt section where the user can specify, in strict terms, the exact folder or subfolder scope that may be worked on
- explicit instruction that if this folder-scope prompt is filled, the agent must treat it as a hard boundary and refuse work outside the specified path set
- a conditional RTK usage rule: if RTK is detected as installed and usable in the environment, the generated `AGENTS.md` must instruct agents to prefer RTK commands for token-efficient shell-based repository reading and inspection workflows based on the capabilities documented in `https://github.com/rtk-ai/rtk`
- the RTK section must not stop at a generic instruction; it must include concrete repository-ready command guidance for the categories RTK actually supports, including at minimum:
  - file and tree inspection such as `rtk ls .`, `rtk read <file>`, `rtk smart <file>`, `rtk find "<glob>" .`, `rtk grep "<pattern>" .`, and `rtk diff <left> <right>` when relevant
  - build and error-focused output reduction such as `rtk err npm run build` or the equivalent build command for the detected stack when RTK supports wrapping that command
  - data-analysis or structured-output inspection workflows only when RTK support is actually evidenced by the repository or installed tool behavior; otherwise the generated text must explicitly say `Needs verification` instead of inventing a command
  - test-runner workflows using concrete RTK-wrapped commands when supported by the detected stack, for example `rtk test cargo test`, `rtk vitest run`, `rtk playwright test`, `rtk pytest`, `rtk go test`, `rtk cargo test`, `rtk rake test`, or `rtk rspec`
- the generated `AGENTS.md` should explain the intent of those RTK examples briefly so the guidance is operational, not just a command list
- explicit fallback rule that if RTK is not installed, not configured, or not applicable to the current operation, agents must continue with the repository's normal supported tools without blocking the task
- explicit note that RTK usage instructions must only cover operations actually supported by RTK and must not invent unsupported RTK write/edit capabilities
- repository-specific warnings
- implementation priorities
- expectation of evidence-based behavior
- instruction that `.agents/` is living documentation and must stay aligned with the current repository state
- conditional rule: if user confirms skills implementation, `AGENTS.md` must include a short section that references `./.agents/skills/` as generated skills artifacts
- explicit structural-change rule: if files or folders are added, renamed, moved, or deleted, the agent must review whether `./.agents/style.md`, `.agents/overview.md`, `./.agents/design.md`, `.agents/components.md`, `./.agents/routes.md`, conditional `./.agents/skills/` (if enabled), and today's file under `./.agents/memory/` need updates in the same task
- explicit rule that if a structural change introduces a new area not yet covered in `.agents/`, the agent must add or extend the relevant agents documentation instead of silently skipping it
- explicit rule that when an agent notices `.agents/` content is outdated relative to the codebase, it must either update it or clearly report the documentation gap
- explicit rule that today's memory file in `./.agents/memory/` must be updated for work performed today using the format `- [file] , [task] , [time]`, except if change in memory file itself then do not add self-referential log entry
- explicit explanation that files under `.agents/memory/` are not auto-updated for every agent action because not every action represents meaningful completed work, intermediate exploration can create noisy or misleading history, and updates should happen when there is a real work result, decision, or completion worth preserving
- concise completion checklist covering:
  - correct file placement
  - reuse of existing building blocks before creating new ones
  - `.agents/` updates after structural changes
  - route or execution-flow updates after entrypoint changes
  - memory log updated for today's work
  - generated context remaining aligned with current repository state
- explicit Definition of Done (DoD) section requiring successful completion of relevant verification gates evidenced in the repository (for example lint, typecheck, and tests when those gates exist) before considering implementation complete

This file should be concise compared to the agents files, but still directive.

### 2. `.agents/overview.md`
Purpose: high-level repository map and execution context.

Must include, adapted to the real stack:
- project snapshot in 1 to 3 paragraphs
- detected primary stack and runtime summary
- architecture summary
- real install / dev / build / verification commands when available
- repository structure snapshot or tree block
- key entry points and source roots
- important configuration files
- high-signal operational notes for agents
- maintenance note that structural repository changes (add, rename, move, delete) require this file's structure snapshot and important directories to be refreshed

### 3. `.agents/style.md`
Purpose: coding conventions and implementation patterns.

Must include, adapted to the real stack:
- naming conventions
- file and folder naming style
- import / include / module conventions
- typing or schema conventions
- component / controller / service / handler / module patterns depending on stack
- error handling style
- shared code placement rules
- examples of preferred patterns based on detected code
- `Avoid` section for patterns that conflict with observed repository style
- maintenance note that when structural changes introduce new naming patterns, folder conventions, import paths, module boundaries, or implementation patterns, this file must be updated to reflect the current dominant style

### 4. `.agents/design.md`
Purpose: visual design-system guidance and UI implementation intent for agents.

Must include, adapted to the real stack:
- visual theme and atmosphere, detected from real UI/theme/config artifacts
- color palette and role mapping, with token or source references when present
- typography rules including family, hierarchy, scale, weight, line-height, and spacing patterns
- component styling patterns for real reusable UI blocks in the repository (for example buttons, forms, cards, navigation, media, carousel)
- layout principles including spacing system, grid/container behavior, and border-radius tendencies
- depth and elevation patterns (shadow, border, overlay, layering) as observed
- responsive behavior including breakpoints, collapse strategy, and component adaptation if detected
- `Do` and `Don't` patterns inferred from observed implementation style
- agent prompt guide for generating new UI consistent with detected design language
- maintenance note that when design tokens, theme files, shared UI primitives, or visual rules change, this file must be updated to match current behavior

### 5. `.agents/components.md`
Purpose: reusable building blocks and structural responsibilities.

Must include, adapted to the real stack:
- responsibility map of major directories or packages
- reusable building blocks before creating new ones
- common abstractions already present
- shared layers, common modules, adapters, utilities, hooks, services, entities, packages, or UI primitives depending on stack
- file placement rules for new code
- examples of how repository code is typically composed
- maintenance note that when new folders, shared modules, reusable components, or other code locations are added, renamed, moved, or deleted, this file must be updated to reflect the current reuse map and placement rules

### 6. `.agents/routes.md`
Purpose: execution flow, entry points, and externally visible boundaries.

Must include, adapted to the real stack:
- route map, command map, API surface map, worker flow, or app navigation map depending on repository type
- top-level entry points
- feature areas or package boundaries
- framework-specific dispatch or composition flow
- skill / workflow trigger notes only if they are actually part of the repository workflow
- guidance for where agents should connect new functionality
- maintenance note that when routes, entry points, handlers, commands, workers, screens, or other execution boundaries are added, renamed, moved, or deleted, this file must be updated to match the current flow

If the repository has no meaningful route or entrypoint map, state that clearly and document the nearest equivalent execution boundary.

### 7. `.agents/memory/` (daily memory files)
Purpose: daily work log and supporting memory context for agents.

File rule:
- Create one file per day inside `./.agents/memory/`
- Naming format: `date-month-year-memory.md` (example: `26-04-2026-memory.md`)
- If date changes, create new file for new day instead of appending previous day's file

Must include:
- a section in today's daily memory file for work log entries using the exact format `- [file] , [task] , [time]`
- entries that record work completed today, focused on concrete file-level activity
- a short description of what qualifies as a `task` entry so the format stays consistent
- a short description of the expected `time` format used in the log
- a section describing what supporting memory is needed alongside the daily log, such as active focus, open follow-up items, decisions made today, blockers, and references needed to continue the work later
- a clear distinction between factual work-log entries and inferred / pending follow-up notes
- a maintenance note that today's memory file must be refreshed whenever work is completed on the same day, and a new file must be created when the date changes
- an explicit explanation of why memory is not auto-updated on every single action: only meaningful completed work, confirmed decisions, blockers, and handoff-relevant notes should be logged, while exploratory reads, temporary checks, retries, and partial steps should usually stay out to keep the memory useful and low-noise

---

## VI. ADAPTATION RULES BY REPOSITORY TYPE

### Frontend / Mobile Apps
Prefer concepts such as:
- components
- screens
- routes
- layouts
- hooks
- styling system
- state management

### Backend APIs / Services
Prefer concepts such as:
- routes
- controllers
- handlers
- services
- repositories
- entities
- migrations
- contracts
- background jobs

### CLI / Automation Tools
Prefer concepts such as:
- commands
- subcommands
- argument parsing
- execution flow
- packages / modules
- adapters
- output formatting

### Libraries / SDKs
Prefer concepts such as:
- public API surface
- packages / modules
- adapters
- examples
- test coverage strategy
- extension points

### Monorepos
Document clearly:
- package boundaries
- app boundaries
- shared package responsibilities
- root vs package-local commands
- cross-package import rules

---

## VII. REQUIRED STYLE OF GENERATED FILES

The generated files should feel concrete and operational, similar in usefulness and depth to the guides in `examples/`.

Each generated file should:
- use clear markdown headings
- include practical bullets
- include fenced code blocks for commands
- include tree blocks for structure where relevant
- include concrete examples based on detected conventions
- be specific rather than generic
- avoid shallow one-line sections
- prioritize actionability for both humans and AI agents

Target output qualities:
- comprehensive
- structured
- explicit
- low-noise
- evidence-based
- implementation-oriented
- stack-aware

Avoid:
- vague phrases like `follow best practices`
- JavaScript-specific assumptions in non-JavaScript repositories
- generic advice that could fit any repository
- undocumented assumptions
- repeated filler across multiple files
- unsupported stack recommendations

---

## VIII. DEPTH REQUIREMENTS

Minimum expectations:
- `overview.md` must be detailed enough to orient a new contributor or AI agent
- `style.md` must be detailed enough to prevent convention drift
- `design.md` must be detailed enough to preserve design intent, token usage, and component-level styling decisions for future agents
- `components.md` must be detailed enough to guide safe code placement and reuse
- `routes.md` must be detailed enough to explain execution boundaries or flow
- `memory.md` must be detailed enough to preserve daily work continuity for future agents
- `AGENTS.md` must be concise but directive

Depth scaling rules:
- If the repository is large, add more structure and examples.
- If the repository is small, stay concise but still specific.
- If multiple apps or packages exist, document boundaries clearly.
- If conventions are inconsistent, document the dominant pattern and note inconsistencies.
- If the repository is backend-only or library-only, adapt agents content names internally without changing file names.

---

## IX. FILE CONTENT QUALITY GATE

Before finalizing the output, verify all of the following:
- every major recommendation maps to repository evidence
- no unsupported library or tool was introduced
- all commands are real or clearly labeled as inferred / needs verification
- architecture description matches the observed directory structure
- naming conventions reflect actual code patterns
- agents files match the detected stack instead of forcing generic frontend language
- workflow guidance is actionable for an AI agent
- generated files are detailed enough to be genuinely useful
- design-system claims in `design.md` map to repository evidence (tokens, theme config, style artifacts, or component implementations)
- missing or weakly evidenced design details are labeled with `Not detected in repository` or `Needs verification`
- skills confirmation decision is reflected correctly: no `./.agents/skills/` output when `No`, and created + referenced + populated via `npx autoskills` when `Yes`
- ambiguous areas are labeled honestly instead of guessed

If any section feels generic, improve it before final output.

---

## X. EXECUTION INSTRUCTIONS

When applying this skill, perform the following sequence:

1. Inspect the repository.
2. Ask user first whether to implement skills generation for this run (`Yes` or `No`) using this exact question:
   - `Implement skills generation for this run? (Yes/No)`
   - treat only `Yes` or `No` as valid direct answers; if answer is unclear, ask once for clarification before continuing.
3. If user answered `Yes` on skills implementation:
   - create `./.agents/skills/`
   - ensure `AGENTS.md` includes reference to `./.agents/skills/`
   - run `npx autoskills` at the end of generation to populate `./.agents/skills/`
4. Before any code add/edit/delete execution, read `./.agents/skills/` first to apply available skills artifacts for the current operation.
   - if `./.agents/skills/` is not present, not generated, or not applicable to the current task, continue without blocking and mark the gap as `Needs verification` where relevant.
5. Extract evidence from manifests, config, structure, and source files.
6. Infer the real stack, architecture, conventions, and workflows.
7. Generate `AGENTS.md`.
8. Generate the `.agents/` documentation pack:
   - `overview.md`
   - `style.md`
   - `design.md`
   - `components.md`
   - `routes.md`
9. Generate `.agents/memory/` daily memory file for current date using naming `date-month-year-memory.md`.
10. Apply TDD flow only when the change adds, edits, or removes application logic/behavior: create/update failing test first, implement minimal code change, then rerun relevant tests until pass.
    - for non-logic changes (for example wording, formatting, comments, docs-only, or mechanical rename without behavior change), TDD is not required.
11. Review the output against the quality gate before finalizing.

Do not skip the inspection step.
Do not skip the skills confirmation question.
Do not produce generic boilerplate.
Do not assume the repo is JavaScript-based.
Do not use unsupported tools or conventions.

---

## XI. EXECUTION PROMPT

Apply the `PROJECT_CONTEXT_ARCHITECT` skill to this repository.

Before generation, ask user whether to implement skills generation for this run (`Yes` or `No`) with this exact prompt:
- `Implement skills generation for this run? (Yes/No)`
- if reply is not explicit `Yes` or `No`, request one clarification and pause generation until explicit answer is provided.

Inspect the real project files, infer the actual stack and conventions from evidence, and generate:
- `AGENTS.md`
- `.agents/overview.md`
- `.agents/style.md`
- `.agents/design.md`
- `.agents/components.md`
- `.agents/routes.md`
- `.agents/memory/` with today's file named `date-month-year-memory.md`
- if user selected `Yes` for skills implementation: create `.agents/skills/` and include generated skills files from `npx autoskills`

If user selected `Yes`, ensure `AGENTS.md` references `./.agents/skills/` and run `npx autoskills` at end of generation.

The output must be repository-specific, highly practical, and adapted to the actual language, framework, and architecture in the repository.

Never hallucinate stack choices, commands, or conventions.
Use `Not detected`, `Needs verification`, or `Inferred from current structure` whenever evidence is missing.
