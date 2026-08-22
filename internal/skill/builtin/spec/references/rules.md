# RULES.md — Template

> Fill every section from the confirmed understanding in Phase 1, scoped to
> the languages/frameworks actually chosen in ARCHITECTURE.md. Every "Do" or
> "Don't" must be concrete enough that an AI agent can check it mechanically.
> Unconfirmed items go in the `⚠ Unresolved assumptions` callout. This file is
> loaded by coding agents — write it as a contract, not advice.

> ⚠ Unresolved assumptions — only present if confidence < 100%. Else delete.

# Development Rules & Coding Standards

> These rules apply to every AI agent and human developer working in this
> repository. They are enforceable, not aspirational.

## 1. Coding Conventions

### Naming

| Construct | Convention | Example |
|---|---|---|
| Variables / functions (TS/JS/Go) | camelCase | `getUserById` |
| Classes / components / types | PascalCase | `UserCard` |
| Files / directories (JS/TS) | kebab-case | `user-card.tsx` |
| DB columns / tables | snake_case | `user_id`, `orders` |
| Constants | UPPER_SNAKE_CASE | `MAX_RETRIES` |

### Type Safety & Modularity

- Strict mode ON for TypeScript (`"strict": true`); no `any`, no `@ts-ignore`
  without a documented reason and issue reference.
- Every function has an explicit return type. No implicit returns of `Promise`.
- One responsibility per function; functions under ~40 lines unless the
  domain requires otherwise. No 1000-line god components.
- No dead code: remove unused imports, variables, and exported symbols on sight.

## 2. Git Workflow & Commit Guidelines

### Conventional Commits

```
feat: add payment webhook handling
fix: resolve session expiry race on refresh
docs: document /spec output contract
refactor: extract order validation into service
test: cover retry path in queue worker
chore: bump dependencies
```

- Format: `<type>(<scope>)?: <imperative summary>` — lowercase, ≤ 72 chars,
  no trailing period.
- Breaking changes: append `!` (e.g. `feat!: drop v1 API`).

### Branching

```
main          # production, protected, only merge via PR
develop       # integration branch
feature/*     # branched from develop, merged via PR
fix/*         # hotfix branches from main for urgent fixes
```

- PRs: one logical change per PR, description explains why, references issue.
- Never push directly to `main` / `develop`.

## 3. Error Handling & Logging

### Standardized API Error Response

Every error response uses the same shape and never leaks internals:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "email must be a valid address",
    "details": ["field: email, reason: invalid format"]
  }
}
```

- `4xx` for client errors, `5xx` for server errors; map domain errors to codes.
- Never return raw exception messages or stack traces to the client.
- `try/catch` at the boundary (handler), not around every statement; let
  errors propagate to a central error handler.

### Logging

- Structured JSON logs (Winston/Pino/std lib), never string concatenation.
- Minimum fields: `timestamp`, `level`, `event`, `request_id`, `duration_ms`.
- Include a `request_id` from request start to response end (X-Request-Id).
- Level discipline: `debug` = dev only, `info` = lifecycle events,
  `warn` = recoverable anomalies, `error` = failed operations.
- Never log secrets, tokens, passwords, or full PII.

## 4. Testing Requirements

- New business logic ships with tests. Minimum: unit coverage ≥ 80% on
  `core/` and `services/`, 100% on pure utility functions.
- Unit tests (Vitest/Jest): fast, no network, no DB. Mock boundaries.
- Integration tests: real DB in container (testcontainers or docker-compose),
  cover the public API surface happy + error paths.
- Test naming: `describe + it('should <expected> when <condition>')` or
  `TestXxx` in Go. Arrange → Act → Assert, one behavior per test.
- Run `lint` + `test` before every commit; CI enforces both.

## 5. Strict Do's and Don'ts

### Never

- NEVER hardcode API keys, secrets, tokens, or credentials in code, config
  committed to git, or logs. Use environment variables / secret manager only.
- NEVER use `any` in TypeScript. If the type is unknown, narrow it.
- NEVER swallow errors (`catch {}`, empty catch, ignoring promise rejections).
  Handle, log, or rethrow — pick one.
- NEVER write unparameterized SQL (`"SELECT * FROM t WHERE id = " + input`).
  Use the ORM or parameterized queries.
- NEVER commit generated artifacts (`node_modules/`, `dist/`, `.env`) —
  gitignore them.
- NEVER modify database schema without a migration file committed alongside.
- NEVER bypass the error response contract to return ad-hoc shapes.
- NEVER push directly to protected branches.

### Always

- ALWAYS add/adjust tests in the same commit as the code change.
- ALWAYS write against the interfaces in ARCHITECTURE.md — no cross-layer
  shortcuts (UI must not touch the DB layer directly).
- ALWAYS break long-running tasks for background workers, not the request
  handler.
- ALWAYS keep secrets out of the repo; document required env vars instead.
- ALWAYS keep the context/ files in sync when architecture decisions change.

## 6. Definition of Done

- [ ] Code follows naming/structure conventions above.
- [ ] Tests written and passing locally + in CI.
- [ ] No secrets, `any`, swallowed errors, or raw SQL concatenation.
- [ ] Commits follow Conventional Commits; PR merged via review.
- [ ] Docs (context/*.md) updated if behavior or architecture changed.