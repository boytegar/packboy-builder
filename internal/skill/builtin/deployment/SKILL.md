---
name: deployment
description: Deployment strategies for Go applications: blue-green, canary, rolling, and serverless. Use when planning or executing application deployments.
origin: builtin
---

# Deployment

## When to use
- Planning a deployment strategy
- Executing a deployment with rollback plan
- Setting up zero-downtime deployments
- Configuring health checks and readiness probes

## Deployment strategies

### Rolling update
- Replace old instances with new ones gradually
- Default in Kubernetes (maxSurge/maxUnavailable)
- Zero downtime with health checks
- Rollback: trigger another rolling update with previous image

### Blue-green
- Two identical environments (blue = current, green = new)
- Deploy to green → test → switch traffic → keep blue as fallback
- Instant rollback: switch traffic back to blue
- Requires double infrastructure

### Canary
- Deploy new version to a small % of instances/traffic
- Monitor error rate, latency, business metrics
- Gradually increase traffic if healthy
- Auto-rollback on anomaly detection

### Feature flags
- Deploy code with new features disabled
- Enable for specific users/percentage
- Can turn off without redeployment
- Decouple deployment from release

## Health checks
- **Liveness**: is the app running? (restart if not)
- **Readiness**: is the app ready to serve traffic? (remove from pool if not)
- **Startup**: is the app starting up? (don't kill for slow startup)
- Return 200 OK when healthy, non-200 otherwise
- Check dependencies (DB, cache) in readiness, not liveness

## Zero-downtime checklist
- [ ] Health checks configured
- [ ] Graceful shutdown (SIGTERM → finish in-flight requests → exit)
- [ ] Database migrations are backward-compatible
- [ ] No sticky sessions (or session migration plan)
- [ ] Rollback plan tested
- [ ] Monitoring and alerting in place

## Graceful shutdown (Go)
```go
srv := &http.Server{Addr: ":8080"}
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}()
<-ctx.Done() // wait for SIGTERM
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx) // finish in-flight requests
```

## Rollback
- **Image rollback**: redeploy previous image tag
- **Database rollback**: only if migrations are reversible (they often aren't)
- **Code rollback**: `git revert` + redeploy
- Test rollback before you need it

## Common anti-patterns
- Deploying without health checks
- No rollback plan
- Destructive migrations in the same deployment as code changes
- Not testing the deployment process in staging
- Big-bang deploys (deploy everything at once vs incremental)
