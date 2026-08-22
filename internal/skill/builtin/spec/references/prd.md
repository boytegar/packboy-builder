# PRD.md — Template

> Fill every section from the confirmed understanding in Phase 1. If a section
> genuinely does not apply, write "N/A — <reason>" instead of deleting it.
> All content must trace to what the user stated or what you verified in the
> repo. Unconfirmed items go in the `⚠ Unresolved assumptions` callout at the
> top of the file.

> ⚠ Unresolved assumptions — only present if confidence < 100% after 3
> questioning rounds. List each open gap explicitly; else delete this block.

# Product Requirements Document

## 1. Product Overview

### Background / Problem
Why does this product need to exist? What pain is it solving? (1-3 sentences.)

### Business Goals
What business outcome does this feature serve? (Revenue, retention, cost, ...)

### Value Proposition
One sentence: what does the user gain that they could not get before?

## 2. Target Users & Personas

### Personas
For each persona:

| Persona | Role | Description | Primary Goals |
|---|---|---|---|
| e.g. Admin | System administrator | Manages users, config, billing | Audit, configure, escalate |

### Roles & Access Rights

| Role | Can do | Cannot do |
|---|---|---|
| Admin | ... | ... |
| Regular User | ... | ... |
| Guest | ... | ... |

## 3. User Stories & Use Cases

Format: `As a [persona], I want to [action] so that [benefit].`

- As an Admin, I want to ... so that ...
- As a Regular User, I want to ... so that ...
- As a Guest, I want to ... so that ...

### Primary Use Case Flow (happy path)
1. Step ...
2. Step ...
3. Step ...

## 4. Functional Requirements

Grouped by module/area. Each requirement: imperative sentence + acceptance
criterion.

| ID | Module | Requirement | Acceptance Criterion |
|---|---|---|---|
| FR-01 | Auth | ... | ... |
| FR-02 | ... | ... | ... |

Include business rule notes where the flow is non-trivial (state transitions,
permission boundaries, idempotency).

## 5. Non-Functional Requirements

| ID | Category | Requirement |
|---|---|---|
| NFR-01 | Performance | e.g. p95 load time < 2s (or: no SLA — small scale) |
| NFR-02 | Capacity | e.g. supports 10k concurrent users |
| NFR-03 | Availability | e.g. 99.9% uptime SLA |
| NFR-04 | Security | e.g. HTTPS everywhere, password hashing with bcrypt/argon2 |
| NFR-05 | Compliance | e.g. GDPR / SOC2 / HIPAA, or "none required" |
| NFR-06 | Observability | e.g. structured logs, error tracking |

## 6. Scope & Out of Scope

### In Scope (this phase)
- ...

### Out of Scope (deferred / explicitly not now)
- ...

## 7. Key Success Metrics (KPIs)

| Metric | Definition | Target |
|---|---|---|
| e.g. Conversion rate | signups that reach first paid action | > 5% |
| e.g. User activation | % accounts completing onboarding | > 60% |
| e.g. Load time | p50 page load | < 1.5s |