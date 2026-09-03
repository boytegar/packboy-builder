---
name: incident-response
description: Incident response and root cause analysis runbook for production alerts. Use when investigating an alert, performing RCA, or documenting post-mortems.
origin: builtin
---

# Incident Response

## When to use
- Production alert triggered
- Root cause analysis needed
- Writing post-mortem documentation
- Building runbooks for future incidents

## Incident lifecycle
1. **Detect**: alert fires, user reports, or monitoring catches anomaly
2. **Triage**: assess severity, page the right people
3. **Mitigate**: stop the bleeding (rollback, scale, circuit break)
4. **Investigate**: find root cause (logs, metrics, traces)
5. **Remediate**: fix the actual issue
6. **Post-mortem**: document, learn, prevent recurrence

## Severity classification
- **SEV1 (Critical)**: total outage, data loss, security breach — page on-call 24/7
- **SEV2 (High)**: significant degradation, no workaround — page on-call
- **SEV3 (Medium)**: limited impact, workaround available — business hours
- **SEV4 (Low)**: minor, cosmetic — ticket

## RCA methodology
- **Timeline**: reconstruct the sequence of events
- **5 Whys**: ask "why" until you reach a systemic cause (not a person)
- **Bisect**: find the change that introduced the issue (git bisect, deploy history)
- **Different perspectives**: code, config, deployment, data, load, dependency
- **Don't stop at the first cause**: dig deeper — "the developer made a mistake" is not a root cause

## What to check during investigation
- **Recent deployments**: what changed? (deploys, config changes, migrations)
- **Logs**: errors, warnings, unusual patterns
- **Metrics**: latency, error rate, throughput, saturation
- **Traces**: where is time/error coming from?
- **Dependencies**: are upstream services healthy?
- **Capacity**: are we out of memory, disk, connections?
- **Data**: was bad data ingested?

## Post-mortem format
```
## Incident: [title]
- **Date**: YYYY-MM-DD
- **Severity**: SEV1/2/3
- **Duration**: X hours
- **Impact**: what users experienced
- **Timeline**: chronology of events and actions
- **Root cause**: the systemic issue (not "human error")
- **Contributing factors**: what made it worse
- **What went well**: effective mitigations
- **What went poorly**: gaps in detection/mitigation
- **Action items**: prevent recurrence (with owners and deadlines)
```

## Anti-patterns
- Blaming individuals ("the developer should have been more careful")
- Stopping at the proximate cause without finding the systemic issue
- No action items (post-mortem theater)
- Action items with no owner or deadline
- Not sharing the post-mortem (learning stays siloed)
