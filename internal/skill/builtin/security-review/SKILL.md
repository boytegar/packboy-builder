---
name: security-review
description: Security-focused code review using STRIDE, OWASP Top 10, and supply chain analysis. Use when reviewing a PR for security vulnerabilities or performing a security audit of code changes.
origin: builtin
---


Security review is the practice of finding exploitable weaknesses in code before they ship.
Unlike a general code review — which weighs design, readability, and correctness — a security
review assumes an adversary is reading the same diff and asking: *what can I make this code do
that the author did not intend?* Every finding here is framed from that attacker perspective.

This skill is opinionated about Go. Packboy Builder (pcb) is a Go CLI/TUI, so the concrete
patterns, sinks, and remediations below are Go-first. Where a pattern is language-agnostic
(STRIDE categories, OWASP Top 10, CVSS scoring, secrets detection), it is stated as such and
then specialized for Go. Apply the language-agnostic framing to non-Go files in the same diff,
but do not invent Go-specific advice for code that is not Go.

## When to use this skill

Use this skill when **any** of the following are true:

- A pull request, branch diff, or patch set is under review and security impact must be
  assessed before merge.
- A user asks for a "security audit", "vulnerability scan", "pentest review", or "threat
  model" of code changes or a whole repository.
- A change touches authentication, authorization, cryptography, file/path handling, shell or
  process execution, network requests, deserialization, configuration/secrets, or dependency
  manifests (`go.mod`, `go.sum`).
- An incident or near-miss has occurred and a root-cause security review of the offending code
  is required.

Do **not** use this skill for:

- General code quality, style, or architecture review with no security dimension — use the
  `code-review` skill instead.
- A full-project, depth-first, multi-model jury audit of a single repository — use the
  `deep-security-review` skill instead. This skill is the lighter, single-pass reviewer suited
  to PR-sized diffs.
- Operational incident response runbooks — use the `incident-response` skill.

If the scope is ambiguous (PR-sized diff vs. whole-repo audit), ask the caller to clarify
before proceeding; the two modes produce very different output sizes.

## Inputs and scope

A security review consumes:

1. **The diff under review** — staged or branch diff, ideally per-file with context. For a
   whole-repo audit, the full source tree. Never review only the commit message; review the
   code that actually changed.
2. **The dependency manifest** — `go.mod` and `go.sum` for Go projects; equivalent manifests
   for other languages present in the repo.
3. **The runtime context, if known** — what privileges the code runs with, what it exposes
   (HTTP server, CLI on a developer machine, daemon), who the intended and untrusted users
   are. If the context is not given, state your assumptions explicitly in the report.

State scope assumptions up front. A review that silently assumes "this is a local CLI trusted
to run anything" will miss vulnerabilities that matter when the same code runs as a networked
service. When in doubt, model the most hostile plausible deployment.

## Review methodology

Run the review as a structured pass, not a free-form read. Order matters: the threat model
frames what you look for; the checklist ensures coverage; the scoring ensures findings are
prioritized rather than dumped in a flat list.

### 1. Build a lightweight threat model first

Before listing findings, identify:

- **Trust boundaries** — where does untrusted data enter the system? (HTTP handlers, CLI args,
  file reads, network responses, environment variables, IPC channels, plugin inputs.)
- **Privileged operations** — what does the code do that an attacker would want to coerce?
  (file writes outside a sandbox, shell execution, network calls to internal hosts, crypto
  operations, reads of secrets, privilege changes.)
- **The attacker's position** — can the attacker reach the input parser directly, or must
  they first authenticate? A finding that requires an authenticated attacker is still a
  finding, but it scores lower than one reachable pre-auth.
- **Assets at risk** — secrets on disk, credentials in memory, user data, system integrity,
  availability of a service.

Record this threat model as the opening "Scope & Threat Model" section of the report. It
frames every subsequent finding and lets the reader judge whether your assumptions match their
deployment.

### 2. Run the STRIDE pass

Apply the STRIDE categories to every changed code path. STRIDE is a coverage tool, not a
theorem: for each category, ask "can the changed code enable this?" and record the answer
(yes/no/suspect) per changed area. Convert "suspect" items into concrete findings in step 3.

### 3. Run the Go-specific and OWASP pass

Walk the Go and OWASP Top 10 checklists (below) against the diff. Each checklist item is a
*question*, not a rule — answer it per changed file. "No, this does not apply because…"
is a valid and valuable answer; it documents that the category was considered, not skipped.

### 4. Run the supply chain pass

Review dependency changes and the manifest. See "Dependency security" below.

### 5. Score, dedupe, and rank findings

Apply CVSS-style scoring to every finding, deduplicate overlaps, and emit the report in
severity order. See "Output format" below.

## STRIDE threat modeling

STRIDE is a mnemonic for the six classes of threats Microsoft's threat-modeling taxonomy
covers. For a code review, treat each category as a *lens*: look at the same diff six times,
once per lens. A single changed function can yield findings under more than one lens.

### Spoofing (S)

Spoofing is impersonating an identity the system trusts. In Go code, look for:

- Authentication checks that are missing or bypassable — a handler that reads a "user" from a
  context or cookie but never validates the credential that put it there.
- Trust derived from client-controlled input: `r.Header.Get("X-User")`, `r.RemoteAddr`
  treated as an identity, a JWT whose `sub` claim is trusted without signature verification.
- Token validation that skips issuer/audience/expiry checks (`jwt.Parse` without a
  `jwt.Keyfunc` that enforces `iss`, `aud`, `nbf`, `exp`).
- TLS verification disabled — `http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}`.
  Flag every occurrence; it is almost never safe in production.
- Mutual TLS or service-account identity assumed but never actually checked.

For each spoofing suspect, identify *whose* identity is being assumed and *what credential*
establishes it. If there is no credential, there is no authentication.

### Tampering (T)

Tampering is unauthorized modification of data or code. In Go, look for:

- Unvalidated writes to paths derived from user input (path traversal — see "Common Go
  security issues").
- State mutated without authorization checks — a PUT/POST handler that updates a record
  without checking that the caller owns it (IDOR / broken object-level authorization).
- Integrity checks missing on downloaded or deserialized data — fetching a binary or archive
  over plain HTTP and executing/extracting it without a checksum or signature.
- Cookies or tokens signed with a weak or absent MAC; state stored in a cookie without
  `http.SecureCookie` or an HMAC.
- Configuration loaded from a world-writable location and trusted without validation.
- SQL or command construction that concatenates user input (covered under injection, but the
  *tampering* lens asks: what can the attacker *change* once injection succeeds?).

### Repudiation (R)

Repudiation is denying an action the system cannot prove happened. In Go, look for:

- Missing or forgeable audit logs — security-relevant actions (login, privilege change,
  secret read, destructive operation) recorded without a trusted actor identity or without
  tamper-evidence.
- Logs that include attacker-controlled input verbatim (enabling log injection — newlines,
  ANSI escapes, or markup that hides a real entry inside a fake one).
- Nonces or request IDs that are predictable or absent, making it impossible to correlate an
  action to a request after the fact.
- Actions taken on behalf of a user without recording *which* credential authorized the action
  (e.g., a service token vs. the user's own session).

Repudiation findings are often lower severity in low-stakes systems and critical in
financial, compliance, or multi-tenant systems — score against the stated asset value.

### Information Disclosure (I)

Information disclosure is leaking data to someone not entitled to see it. In Go, look for:

- Secrets in source or configuration — API keys, passwords, private keys committed to the
  repo or embedded in a binary. See "Secrets detection".
- Verbose error messages that echo sensitive internals — stack traces, file paths, SQL,
  environment values returned to the client.
- Path traversal or directory listing that exposes files outside an intended root.
- Logging of secrets, tokens, PII, or full request bodies at INFO/DEBUG level that ship to a
  shared sink.
- Insecure responses — `Cache-Control`/`Content-Type` headers missing on endpoints that
  return sensitive data, enabling browser or CDN caching of private responses.
- Side channels — timing differences in auth checks (string compare instead of
  `subtle.ConstantTimeCompare`), or error-message divergence that reveals whether a
  username exists.
- Mass-assignment — binding client input directly onto a struct that contains privileged
  fields (`IsAdmin`, `Role`, `UserID`), letting a client set fields it should not.

### Denial of Service (DoS)

Denial of service is degrading or crashing a service for legitimate users. In Go, look for:

- Unbounded resource consumption — reading a request body with `io.ReadAll(r.Body)` with no
  `http.MaxBytesReader` limit; `json.Unmarshal` of attacker-controlled payloads of arbitrary
  size; recursion without a depth bound.
- Algorithmic complexity attacks — parsing structures (nested JSON, regex, YAML) where deep
  nesting or pathological input causes super-linear time or stack growth.
- Goroutine or connection leaks — spawning a goroutine per request unit without a cap, or
  accepting connections without a `Listener` backlog limit; a slowloris-style attacker can
  exhaust goroutines.
- Blocking operations on the main path — long-held locks, unbounded `select`, or synchronous
  I/O that a flood of requests can pile up behind.
- File or socket exhaustion — opening files per-request without closing on all paths
  (including `defer` ordering issues), leaking file descriptors until `ulimit -n` is hit.
- Panic surface — a nil dereference or index-out-of-range reachable from untrusted input can
  crash a goroutine; if the goroutine is the request handler, the request fails. If it is a
  shared manager, the process can die.

For Go specifically, note that a panic in a request handler goroutine crashes only that
request *unless* it is recovered at a boundary that is shared; but a panic in `main`'s
goroutine or in a long-lived manager goroutine takes the whole process. Score accordingly.

### Elevation of Privilege (EoP)

Elevation of privilege is gaining capabilities the attacker did not have. In Go, look for:

- Authorization checks missing or incorrectly ordered — authn without authz, or authz checked
  before authn (so an unauthenticated user reaches the authz branch).
- IDOR / broken object-level authorization — access by ID without ownership check.
- `exec.Command` with any user-influenced argument (see "Common Go security issues"),
  especially when the process runs with elevated privileges or in a container with broader
  access than the caller.
- `setuid`/`setgid` semantics, `sudo` invocations from the code, or capability-bearing
  binaries — flag and scrutinize.
- Sudo-style "do a thing as root" helpers that accept a path or command from a less-privileged
  caller.
- Privilbedoctor escalation via a shared, writable dependency or plugin path (ties into
  supply chain).
- Mass-assignment of privileged fields (overlaps Information Disclosure).

## OWASP Top 10 for Go applications

The OWASP Top 10 is a prioritized list of web-application risk categories. Map each category
to Go-specific review questions. Even when the code under review is not a web server, many
categories still apply (CLI tools handle arguments, config, and files analogously to HTTP
inputs).

1. **A01 Broken Access Control** — Are object-level authorization checks present on every
   handler that reads or mutates a resource by ID? Are role checks enforced server-side, not
   hidden in the client? Are direct-object references (paths, keys) validated against the
   caller's identity?
2. **A02 Cryptographic Failures** — See "Cryptographic review". Are secrets encrypted at rest
   and TLS in transit? Are weak algorithms (MD5, SHA1 for security, DES, RC4) or home-rolled
   schemes present? Are keys rotated and not hardcoded?
3. **A03 Injection** — See "Common Go security issues". Are queries parameterized? Is
   `exec.Command` invoked with any user-influenced argument? Is shell metacharacter handling
   sound? Are template injections or regexp-injection patterns present?
4. **A04 Insecure Design** — Is there a threat model? Are security-relevant flows designed
   with abuse cases in mind, or only happy paths? Are rate limits, depth limits, and
   transaction limits present where the design demands them?
5. **A05 Security Misconfiguration** — Are default credentials or sample accounts present?
   Is debug/error verbosity left on in production defaults? Are directory listings, CORS
   `*`, or overly broad headers present? Are admin endpoints unauthenticated?
6. **A06 Vulnerable and Outdated Components** — See "Dependency security". Are pinned,
   audited dependency versions used? Are known-vulnerable versions present in `go.mod`?
7. **A07 Identification and Authentication Failures** — Are session IDs strong and rotated
   on login/privilege change? Are credentials stored with a strong, salted, slow hash
   (bcrypt/argon2) and *not* MD5/SHA? Are brute-force protections (lockout, rate limit)
   present? Are weak password policies enforced?
8. **A08 Software and Data Integrity Failures** — Are auto-update or plugin mechanisms
   integrity-checked (signature, checksum) before use? Are CI/CD pipelines protected against
   poisoned inputs? Is deserialization of untrusted data bounded and typed?
9. **A09 Security Logging and Monitoring Failures** — Are security-relevant events logged
   with enough context to investigate? Are logs tamper-evident or shipped to a sink the
   attacker cannot reach? Are alerts wired to the events that matter?
10. **A10 Server-Side Request Forgery (SSRF)**** — See "Common Go security issues". Are
    outbound URLs validated against an allowlist? Are internal link-local / loopback /
    metadata-endpoint ranges blocked? Is DNS rebinding considered?

## Common Go security issues

These are the concrete sinks and footguns that recur in Go code. For each, give the pattern,
why it is dangerous, the fix, and a grep-shaped probe you can run against the diff.

### Command injection via `exec.Command`

Pattern (unsafe):
```go
cmd := exec.Command("sh", "-c", "git log "+userInput)
cmd := exec.Command("git", "log", userInput) // better, but still unsafe if userInput is "-flag" or a path traversal
```
Why dangerous: the first form is classic shell injection. The second form avoids the shell
but still lets the attacker inject *arguments* (`--upload-pack=...`, `--output=../../etc/...`)
or hijack subcommands via `git`'s `GIT_*` environment if the environment is inherited.

Fix:
- Prefer the no-shell form `exec.Command("git", "log", "--", safePath)` where `safePath` is
  validated against an allowlist or normalized and confined to a root.
- Scrutinize *every* argument, not just the obvious one — flags can be injected anywhere.
- Pass an explicit, minimal environment with `cmd.Env` when the child may be influenced by
  attacker-set env (`GIT_DIR`, `LD_PRELOAD`, `PATH`).
- Drop privileges before spawning; never run attacker-influenced commands as root or in a
  privileged container.

Probe: `exec\.Command` and `os/exec` imports; any `"sh", "-c"` literal; any `CommandContext`
with a variable in the argument list.

### Path traversal

Pattern (unsafe):
```go
p := filepath.Join(root, userInput) // userInput can be "../../etc/passwd"
data, _ := os.ReadFile(p)
os.WriteFile(filepath.Join(uploadDir, header.Filename), ...) // Filename can contain "../"
```
Why dangerous: `filepath.Join` *normalizes* but does not *confine*. `Join("/var/app", "../../etc/passwd")`
returns `/etc/passwd`. `header.Filename` from a multipart upload is fully client-controlled
and routinely contains `../` or absolute paths.

Fix:
- After joining, verify the result is still inside the root:
  ```go
  abs, _ := filepath.Abs(filepath.Join(root, userInput))
  rel, err := filepath.Rel(root, abs)
  if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
      return fmt.Errorf("path escapes root")
  }
  ```
- For uploads, generate the storage name server-side (a UUID or hash), never the client's
  filename.
- Reject absolute paths and any segment containing `..` before joining.

Probe: `filepath\.Join` with a variable second arg; `os\.Open|ReadFile|Create|WriteFile`
with a joined path; `r\.MultipartReader`, `FormFile`, `header\.Filename`.

### SQL injection

Pattern (unsafe):
```go
db.Query(fmt.Sprintf("SELECT * FROM users WHERE name = '%s'", name))
db.Query("SELECT * FROM users WHERE name = '" + name + "'")
```
Why dangerous: classic injection. The database sees attacker-controlled SQL.

Fix:
- Always parameterize: `db.Query("SELECT * FROM users WHERE name = ?", name)`.
- For dynamic identifiers (table/column names, which cannot be bound), use an allowlist:
  ```go
  if !allowedColumns[col] { return fmt.Errorf("unknown column") }
  query := fmt.Sprintf("SELECT %s FROM t", col)
  ```
- Never build a query by string concatenation, even for "internal" callers — the boundary
  will move.

Probe: `fmt\.Sprintf.*SELECT|INSERT|UPDATE|DELETE`, `db\.Query\(` followed by `+`, `Exec\(`.

### SSRF (Server-Side Request Forgery)

Pattern (unsafe):
```go
resp, _ := http.Get(userURL) // userURL can be http://169.254.169.254/latest/meta-data/
```
Why dangerous: the server becomes a confused deputy, fetching internal resources on the
attacker's behalf — cloud metadata endpoints, internal admin APIs, loopback services.

Fix:
- Validate the URL scheme and host against an allowlist; reject non-`http`/`https`.
- Resolve the host and reject link-local (`169.254.0.0/16`), loopback (`127.0.0.0/8`,
  `::1`), private (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`), and any
  cloud metadata endpoint ranges *after* DNS resolution. Use a custom `net.Dialer` with a
  `Control` hook to re-check the resolved address immediately before connect (defeats DNS
  rebinding).
- Disable redirects or re-validate the target on each redirect.
- Pin a short timeout and a custom client; never reuse the default `http.DefaultClient`.

Probe: `http\.Get|Post|DefaultClient|NewRequest` with a variable URL; any handler that takes
a URL from the request body or query string.

### Insecure deserialization

Pattern (unsafe):
```go
var x map[string]any
json.Unmarshal(body, &x) // into interface{}, then type-assert blindly
gob.Decode(...) // gob of untrusted data
```
Why dangerous: while `encoding/json` does not execute methods on decode (unlike some
serializers), decoding into `interface{}` and then type-asserting lets an attacker shape the
runtime type graph and trigger panics or logic bugs. `gob` is more dangerous: it can
instantiate types the application did not expect. `yaml` decoders vary in safety by library.

Fix:
- Decode into a concrete struct, not `interface{}`; reject unknown fields with
  `Decoder.DisallowUnknownFields()`.
- Bound input size before decoding (`io.LimitReader`).
- Avoid `gob` for untrusted input; prefer JSON with a strict schema.
- For YAML, prefer a library that does not instantiate arbitrary types; bound depth.

Probe: `json\.Unmarshal.*interface`, `gob\.`, `yaml\.Unmarshal`.

### Other recurring Go footguns

- **`math/rand` for security** — using `math/rand` for tokens, IDs, or keys. Use
  `crypto/rand`; flag `math/rand` near any security-relevant value.
- **Weak string compare for secrets** — `token == expected` leaks via timing. Use
  `subtle.ConstantTimeCompare`.
- **`reflect` on untrusted input** — can instantiate unexpected types; avoid.
- **`unsafe`** — any use of `unsafe` near untrusted input is a finding by default; demand a
  written justification.
- **goroutine leaks on unbounded input** — `for { go handle(conn) }` without a cap.
- **`defer` in loops** — resources not released until function return, leaking FDs under load.
- **`os.Getenv` for secrets in containers without verification** — fine if the deployment
  guarantees it; a finding if the same binary is shipped to run on developer laptops.
- **`text/template` with attacker-controlled templates** — can call methods on exposed
  objects; treat as code execution surface.

## Authentication and authorization review patterns

Review authn and authz as a *pipeline*, in the order the request sees them:

1. **Identity establishment** — what credential proves who the caller is? (session cookie,
   bearer token, mTLS cert, signed JWT, API key.) Is the credential *verified*, not just
   *read*? A cookie that is read but not MAC-checked is not authentication.
2. **Session validity** — is the session expired? Rotated after login? Invalidated on logout
   and on privilege change? Is the session ID generated with `crypto/rand` and of sufficient
   length (≥128 bits)?
3. **Authentication vs. authorization** — authn confirms *who*; authz confirms *what they may
   do*. A review that stops at authn misses IDOR. Check that every object access is authorized
   against the authenticated identity, not just that the caller is *some* valid user.
4. **Object-level authorization (IDOR)** — for every handler keyed by an ID (`/users/:id`,
   `/orders/:id`), verify the handler checks that the caller owns or may access that ID.
   Absent check = high-severity finding in any multi-user system.
5. **Privilege checks ordering** — authz must run *after* authn and *before* the action. A
   common bug is checking authz on a code path reachable before authn completes.
6. **Privilege escalation via parameter** — mass-assignment: binding request JSON onto a
   struct that includes `Role`, `IsAdmin`, `UserID`. Use field allowlists (`struct` tags with
   a separate input struct) rather than exposing the persistence model directly.
7. **Token transport** — are tokens sent only over TLS? Are cookies `Secure; HttpOnly;
   SameSite`? Is `SameSite=None` used only with a justification?
8. **Password storage** — are passwords stored with bcrypt/argon2/scrypt, *not* MD5/SHA1/SHA256
   of the password? Is a per-password salt used (modern hashes embed it, but verify)?
9. **Brute-force resistance** — rate limit or lockout on authn endpoints; constant-time
   compare; identical error messages for "wrong user" and "wrong password".
10. **Multi-tenant isolation** — in shared-tenant systems, is tenant a dimension of every
    query? A missing `WHERE tenant_id = ?` is a cross-tenant leak.

## Cryptographic review

Cryptography is where "looks fine" is not fine. Review cryptography defensively and never
approve a home-rolled primitive.

### Weak algorithms and constructions

- **Hashing** — MD5, SHA1 for any security purpose (passwords, signatures, integrity).
  SHA-256 minimum; SHA-3 or BLAKE2 for new code. For passwords, use bcrypt/argon2id/scrypt,
  never a raw hash.
- **Symmetric** — DES, 3DES, RC4, Blowfish for new code. AES-GCM (or ChaCha20-Poly1305) for
  authenticated encryption. Never AES-CBC without a separate MAC (and prefer GCM to avoid the
  MAC footgun). Never ECB.
- **Asymmetric** — RSA with <2048-bit keys, RSA without OAEP/PSS, DSA, ECDSA on weak curves.
- **Random** — `math/rand` anywhere near a secret. Use `crypto/rand`.
- **KDF** — PBKDF1, raw single-round hashing as a KDF. Use argon2id, scrypt, or PBKDF2 with
  a high iteration count.
- **TLS** — versions below TLS 1.2; `InsecureSkipVerify: true`; weak cipher suites; disabled
  renegotiation checks.

### Hardcoded keys and secrets

- Any literal that looks like a key, password, or token in source: flag and demand it move
  to a secret manager or environment injection. Run the secrets-detection pass.
- Test keys that are also valid in production: a finding. Test fixtures must not be the same
  key the binary ships with.
- Key derivation that mixes a hardcoded pepper with a stored salt *and* the pepper is in
  source: the pepper adds little; note it.

### Nonce and IV reuse

- AES-GCM and ChaCha20-Poly1305 are catastrophically broken if a nonce is reused under the
  same key. Nonces must be unique per encryption under a key — use a counter, or derive per
  message with `crypto/rand` and a key that is never reused.
- CBC IVs must be unpredictable; a fixed or all-zero IV is a finding.
- Random nonces drawn with `math/rand`: critical finding.

### Other crypto concerns

- Constant-time compares for MACs and token equality (`subtle.ConstantTimeCompare`);
  non-constant-time compare on a secret is a finding.
- Key rotation: is there a mechanism, or is the key immortal? For long-lived systems, flag
  the absence of rotation as a design finding.
- RNG seeding: `rand.Seed(time.Now())`-style seeding of `math/rand` near security code is a
  smell; verify the security-sensitive values use `crypto/rand`.

## Input validation and sanitization

Input validation is the first line of defense, but it is not a single check — it is a layered
discipline. Review it as four layers:

1. **Decode safely** — bound the input size (`http.MaxBytesReader`, `io.LimitReader`); reject
   oversized or deeply nested payloads; set read timeouts.
2. **Validate structurally** — decode into a concrete struct with required fields, types, and
   ranges. Use `Decoder.DisallowUnknownFields()` where appropriate. Validate with a library
   (`go-playground/validator`) or explicit checks — but the checks must exist.
3. **Normalize then canonicalize** — trim, lowercase, Unicode-normalize, resolve paths — *then*
   re-validate on the canonical form. Path traversal and Unicode-confusable attacks live in
   the gap between raw and canonical input.
4. **Encode on output, not on input** — "sanitization" applied at input time is fragile because
   the right encoding depends on the *sink* (HTML, SQL, shell, JSON). Validate at the input
   boundary; encode at the output boundary. Do not store "sanitized" data — store the original
   and encode when rendering.

Common Go validation smells:

- `r.ParseForm()` then `r.FormValue(...)` used directly in SQL or shell without validation.
- `strconv.Atoi` called but the error ignored, then the zero value used as an ID.
- `regexp` applied to input with an unbounded backtracking pattern (ReDoS).
- `url.Parse` called and `Host` used without checking the scheme or the resolved IP (SSRF).
- `mime` or `Content-Type` trusted from the client for security decisions.
- File uploads accepted by extension rather than by content sniff (`http.DetectContentType`)
  and an allowlist.

## Secrets detection

Scan the diff for material that should not be in source. Treat each hit as a finding even if
the key is a "test" key — verify whether it is genuinely test-only.

### Patterns to look for

- **High-entropy strings** — long base64/hex blobs near names like `key`, `secret`, `token`,
  `password`, `apikey`, `BEGIN PRIVATE KEY`.
- **PEM blocks** — `-----BEGIN RSA PRIVATE KEY-----`, `-----BEGIN EC PRIVATE KEY-----`,
  `-----BEGIN PRIVATE KEY-----`, `-----BEGIN OPENSSH PRIVATE KEY-----`.
- **Known service key prefixes** — `AKIA` (AWS access key), `ghp_`/`gho_`/`github_pat_`
  (GitHub), `xox` (Slack), `AIza` (Google API), `sk-` (OpenAI/Stripe-style).
- **URLs with credentials** — `https://user:pass@host`.
- **`.env` and config files** committed or newly added — `.env`, `config.local.*`,
  `secrets.yaml`, `*.pem`, `*.key`.
- **Hardcoded credentials in test fixtures** — verify they are not also valid in a real
  environment.

### What to report

For each secret hit, report: the file and line, the *type* of secret (AWS key, private key,
generic token), whether it appears live or revoked (note that you cannot verify from the diff
alone — recommend the team rotate and treat as compromised), and the remediation (move to a
secret manager, rotate, add to `.gitignore` and `gitleaks`/`trufflehog` config).

Never print the full secret in the report. Mask to the first and last few characters only;
the report itself becomes an artifact that may be shared.

## Dependency security

Review dependency changes as part of the diff. A new dependency is a new attack surface.

### What to check

- **`go.mod` / `go.sum` diffs** — new modules, version bumps, replaced or retracted modules.
  For each new or bumped module, note the module path, version, and whether the version is the
  latest available.
- **Known-vulnerable versions** — cross-reference the module/version against known advisories
  (the Go vulnerability database `vuln.go.dev`, GHSA, CVE). If the version is pinned to a
  vulnerable release, that is a finding; report the CVE/GHSA, the fixed version, and the
  severity.
- **Replacements and `replace` directives** — `replace` pointing at a fork, a local path, or
  a non-standard module proxy is a supply-chain signal; demand a written justification.
- **Module path legitimacy** — typosquats and dependency-confusion: a module name that
  imitates a popular one, or an internal-sounding name that resolves to a public namespace.
  For private module names, verify they resolve to the intended private proxy, not a public
  one that an attacker could register.
- **Indirect transitive risk** — a small direct dep can pull a large transitive tree. Note
  unusually large or unusual transitive additions.
- **Pinning philosophy** — is the project pinned (`go.sum` provides integrity) and are
  checksums verified? A finding is *not* "uses go modules"; a finding is "pins a known-bad
  version" or "uses a `replace` to an unverified source".

### Tooling to recommend (do not run blindly)

Recommend, where appropriate: `govulncheck` for Go vulnerability scanning, `gitleaks` or
`trufflehog` for secrets in history, `nancy` or `osv-scanner` for broader dependency scanning,
and dependency-review GitHub Actions for PR-time checks. Run these in CI, not ad hoc.

## CVSS-style scoring

Every finding gets a severity score so the report is prioritized, not a flat list. Use a
CVSS-style vector and a derived severity band. The goal is comparability across findings, not
a claim of CVSS expertise.

### Base metrics (use the CVSS 3.1 rubric, summarized)

- **Attack Vector (AV)** — Network (N), Adjacent (A), Local (L), Physical (P). A network-
  reachable finding scores higher.
- **Attack Complexity (AC)** — Low (L) or High (H). "Send a crafted request" is Low;
  "win a race + know an internal token" is High.
- **Privileges Required (PR)** — None (N), Low (L), High (H). Pre-auth findings score higher.
- **User Interaction (UI)** — None (N) or Required (R).
- **Scope (S)** — Unchanged (U) or Changed (C). Changed scope (crosses a security boundary)
  raises severity.
- **Confidentiality (C)**, **Integrity (I)**, **Availability (A)** — None (N), Low (L),
  High (H).

### Severity bands

Translate the base score into a band and treat the band as the headline:

- **Critical** — 9.0–10.0. RCE, auth bypass, secret disclosure that grants full system
  compromise.
- **High** — 7.0–8.9. SQL injection, IDOR at scale, path traversal to arbitrary files, SSRF
  to internal metadata.
- **Medium** — 4.0–6.9. Weak crypto with no demonstrated exploit, missing rate limit on a
  non-critical endpoint, verbose error messages leaking internals.
- **Low** — 0.1–3.9. Missing security header, log verbosity, theoretical issues with no
  plausible exploit path.
- **Informational** — 0.0. Hardening suggestions, defense-in-depth, missing best practice
  with no direct exploit.

### Scoring discipline

- Score the finding *as the diff ships*, not as the code might evolve. If a finding is
  latent (the vulnerable code path is not yet reachable from untrusted input), lower the
  score and note "latent; will become High when caller X is added".
- Do not inflate a finding to Critical to get attention. Inflated severities train the team
  to ignore the band. Score honestly.
- Where you are unsure whether a path is reachable, score conservatively (assume reachable)
  but label the finding "Reachability: needs verification" so the team can confirm or refute.

## Output format

Emit the report in this shape. Severity-sorted, deduplicated, and self-contained.

```
# Security Review: <branch or scope>

## Scope & Threat Model
- Trust boundaries: ...
- Privileged operations: ...
- Attacker position: ...
- Assets at risk: ...
- Assumptions: ... (state any deployment assumptions)

## Summary
- Findings: <count> Critical, <n> High, <n> Medium, <n> Low, <n> Informational
- Top risk: <one-line description of the highest-severity finding>

## Findings (severity order)

### [CRITICAL] <short title> — <CWE or STRIDE category>
- **File:** path/to/file.go:Lstart-Lend
- **Sink:** the unsafe line or pattern, quoted
- **Why:** what an attacker can do, concretely
- **Reachability:** confirmed / needs verification / latent (caller not yet wired)
- **CVSS:** AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H  → 9.8 Critical
- **Fix:** the recommended remediation, with a code sketch where useful
- **References:** CWE-xx, OWASP A0x, relevant advisory URL if any

### [HIGH] ...

### [MEDIUM] ...

### [LOW] ...

### [INFORMATIONAL] ...

## Dependency review
- New modules: <list, each with version and latest>
- Known advisories: <CVE/GHSA or "none found">
- Replacements/`replace`: <list or "none">
- Recommendations: <tooling / CI checks>

## Secrets scan
- Hits: <masked list — file:line, type, status>
- Recommendation: rotate / move to secret manager / add to .gitignore

## Coverage checklist (STRIDE)
- Spoofing: considered — <one-line summary>
- Tampering: considered — ...
- Repudiation: considered — ...
- Information Disclosure: considered — ...
- Denial of Service: considered — ...
- Elevation of Privilege: considered — ...

## Coverage checklist (OWASP Top 10)
- A01 Broken Access Control: ...
- ... (one line each)

## Not reviewed / out of scope
- <explicit list of areas not assessed, so the reader knows the boundary>
```

### Output rules

- **Severity order, descending.** Critical first; Informational last.
- **Deduplicate.** If one root cause produces three symptoms, file one finding with three
  symptom locations, not three findings.
- **Quote the unsafe code.** A finding that does not show the line is not actionable.
- **Give the fix.** A finding without a remediation is half a finding.
- **Never print full secrets.** Mask.
- **Mark uncertainty.** "Reachability: needs verification" is a required field when not
  confirmed; do not omit it to look confident.
- **Stay in scope.** Do not review files outside the diff unless the finding's reachability
  demands tracing into a caller. If you must read outside the diff, say so in the finding.
- **No false certainty.** If a category was considered and clean, say "considered — none
  found". If it was not considered (e.g., no network code in scope, so SSRF N/A), say so.

## Working with this skill

- Be adversarial in analysis, neutral in prose. The report reads as findings, not as
  accusations; phrase every finding as "this code enables X" not "the author did Y".
- Prefer concrete reachability over abstract risk. "Untrusted input reaches this sink via
  handler H → service S → line L" beats "this function is vulnerable".
- When the diff is small, the STRIDE and OWASP checklists are short — do not pad them. A
  one-line "considered, none found" per category is correct and sufficient.
- When the diff is a whole-repo audit, group findings by subsystem so the reader can act
  file-by-file; keep the severity-sorted master list as the index.
- Coordinate with the `code-review` skill when the same diff needs both passes: this skill
  owns security findings; `code-review` owns design and quality. Do not duplicate.
- If the review surfaces a finding whose severity warrants it (Critical/High, confirmed
  reachable, in a deployed path), recommend the team open an incident or use the
  `incident-response` skill for root-cause work — do not let the finding live only in the
  PR thread.
- Never modify code or push remediations. This skill reviews and reports; the team fixes.
