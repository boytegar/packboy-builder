---
name: release-management
description: Release management for Go projects: semantic versioning, changelog generation, release artifacts, and distribution. Use when preparing a release or managing versioning strategy.
origin: builtin
---

# Release Management

## When to use
- Preparing a Go project release
- Managing semantic versioning
- Generating changelogs
- Building and distributing release artifacts

## Semantic versioning (SemVer)
Format: `MAJOR.MINOR.PATCH`
- **MAJOR**: incompatible API changes (breaking)
- **MINOR**: new functionality (backward-compatible)
- **PATCH**: bug fixes (backward-compatible)
- Pre-release: `v1.0.0-alpha`, `v1.0.0-beta.1`, `v1.0.0-rc.1`
- Build metadata: `v1.0.0+20260101`

## Release checklist
1. All tests pass on default branch
2. All planned PRs merged
3. CHANGELOG updated
4. Version bumped (in source and git tag)
5. Cross-compiled binaries built
6. Docker images pushed
7. GitHub release created with artifacts
8. Release notes published

## Changelog format (Keep a Changelog)
```markdown
# Changelog

## [Unreleased]

## [1.2.3] - 2026-09-03
### Added
- New feature X
### Changed
- Improved Y performance
### Fixed
- Bug in Z
### Deprecated
- Old API W (will be removed in 2.0.0)
```

## Cross-compilation (Makefile)
```makefile
release: ## Cross-compile for all platforms
	@for os in darwin linux windows; do \
		for arch in amd64 arm64; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags="-s -w" -o bin/app_$$os_$$arch ./cmd/app; \
		done; \
	done
```

## Go-specific release tips
- `CGO_ENABLED=0` for static binaries (cross-compile friendly)
- `-ldflags="-s -w"` strips debug info (smaller binary)
- `-ldflags="-X main.version=v1.0.0"` inject version at build time
- Use `goreleaser` for automated releases

## Git tags
- Tag format: `v1.0.0` (always with `v` prefix)
- Annotated tags: `git tag -a v1.0.0 -m "Release v1.0.0"`
- Tags are immutable — never force-push or delete released tags
- Tag on the default branch after merge

## Distribution channels
- **GitHub Releases**: binaries + checksums
- **Docker Hub/registry**: `docker push app:v1.0.0`
- **Homebrew tap**: for macOS CLI tools
- **AUR**: for Arch Linux
- **APT/YUM**: for Debian/RedHat (via fpm or goreleaser)

## Pre-release checklist
- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` empty
- [ ] CHANGELOG updated
- [ ] Version constant bumped
- [ ] No `TODO` or `FIXME` in release-critical code
- [ ] Dependencies audited (`govulncheck`)
- [ ] Binary tested on target platforms
