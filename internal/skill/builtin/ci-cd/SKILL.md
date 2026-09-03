---
name: ci-cd
description: CI/CD pipeline design for Go projects: GitHub Actions, GitLab CI, testing gates, build artifacts, and deployment automation. Use when setting up or improving CI/CD pipelines.
origin: builtin
---

# CI/CD

## When to use
- Setting up a new CI pipeline
- Improving existing CI performance
- Adding deployment automation
- Configuring quality gates

## Pipeline stages (canonical order)
1. **Lint** — `gofmt -l`, `goimports`, `golangci-lint`, `go vet`
2. **Build** — `go build ./...` (compilation check)
3. **Unit tests** — `go test -race -count=1 ./...`
4. **Integration tests** — `go test -tags=integration ./tests/...`
5. **Coverage** — `go test -cover -coverprofile=coverage.out`
6. **Security scan** — `gosec`, `trivy` for dependencies
7. **Build artifacts** — cross-compile, Docker images
8. **Deploy** — only on default branch, after all gates pass

## GitHub Actions example
```yaml
name: CI
on:
  push:
    branches: [main, development]
  pull_request:

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: gofmt -l .
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v6

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go test -race -count=1 ./...
      - run: go test -coverprofile=coverage.out ./...

  build:
    needs: [lint, test]
    strategy:
      matrix:
        goos: [linux, darwin, windows]
        goarch: [amd64, arm64]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} go build -o bin/app ./cmd/app
```

## CI best practices
- **Fail fast**: lint before test, test before build
- **Cache dependencies**: `actions/cache` for Go module cache, BuildKit cache mounts
- **Parallelize**: matrix builds for cross-compilation, split test suites
- **Pin versions**: don't use `@latest` for actions — pin to SHA
- **Minimal permissions**: `permissions: contents: read` by default
- **Separate concerns**: lint/test/build are independent jobs that can run in parallel

## Quality gates
- Format check: `gofmt -l .` (must output nothing)
- Lint: `golangci-lint run` (zero warnings)
- Test: all tests pass with `-race`
- Coverage: minimum threshold (e.g., 80% for core packages)
- Security: no high-severity findings from `gosec`

## Release pipeline
- Tag-triggered: `git tag v1.0.0` → release build
- Semantic versioning: `vMAJOR.MINOR.PATCH`
- Changelog generation from commit messages
- Cross-compile binaries for all platforms
- Push Docker images with version tags
- Create GitHub release with artifacts

## Deployment patterns
- **Blue-green**: two environments, switch traffic
- **Canary**: deploy to small % of instances, monitor, roll out
- **Rolling**: replace instances one at a time (Kubernetes default)
- **Recreate**: stop old, start new (downtime, but simplest)

## Secrets management
- Never commit secrets — use CI secrets or vault
- Mask sensitive output in logs
- Rotate secrets regularly
- Use OIDC for cloud auth (no long-lived keys)

## Performance
- Cache Go modules: `~/go/pkg/mod`
- Cache BuildKit: `~/.cache/go-build`
- Use `go test -p N` to parallelize test packages
- Use `gotestfmt` for readable CI output
- Split slow integration tests into a separate job
