---
name: deep-security-review
description: Thorough depth-first security audit of a single repository. Use when a comprehensive security audit is needed beyond a PR review.
origin: builtin
---

# Deep Security Review

## When to use
- Full-project security audit (not just a PR)
- Compliance-driven security assessment
- Pre-release security gate

## Methodology
1. **Scope**: identify all source files, configs, deployment scripts
2. **Threat model**: identify attack surfaces, trust boundaries, data flows
3. **Source audit**: review every source file for vulnerabilities
4. **Dependency audit**: check all dependencies for known CVEs
5. **Secret scan**: search for hardcoded credentials
6. **Config audit**: check deployment configs for misconfiguration
7. **Report**: severity-sorted findings with evidence and remediation

## OWASP Top 10 coverage
1. Broken Access Control — check authz on every endpoint
2. Cryptographic Failures — weak algorithms, hardcoded keys
3. Injection — SQL, command, LDAP, XPath
4. Insecure Design — missing rate limits, no threat modeling
5. Security Misconfiguration — default creds, verbose errors
6. Vulnerable Components — outdated dependencies
7. Authentication Failures — weak password policy, no MFA
8. Software/Data Integrity — unsigned updates, untrusted deserialization
9. Logging Failures — missing audit logs, sensitive data logged
10. SSRF — unvalidated outbound requests

## Output format
```
## FINDING: [severity] [title]
- **File**: path/to/file.go:42
- **Description**: what's wrong
- **Impact**: what an attacker can do
- **Evidence**: code snippet or command output
- **Remediation**: how to fix
- **CVSS**: score and vector
```

## Severity levels
- **Critical**: immediate exploitation, data breach or RCE
- **High**: exploitable, significant impact
- **Medium**: requires conditions or limited impact
- **Low**: defense in depth, not directly exploitable
- **Info**: best practice, no direct vulnerability
