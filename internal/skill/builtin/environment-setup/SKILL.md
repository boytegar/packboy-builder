---
name: environment-setup
description: Development environment setup for Go projects: tool installation, Go workspace, editor config, and dev tooling. Use when setting up a new dev environment or onboarding.
origin: builtin
---

# Environment Setup

## When to use
- Setting up a new Go development environment
- Onboarding a new developer to a Go project
- Configuring editor and tooling
- Troubleshooting Go environment issues

## Go installation
- Install via official installer or package manager
- Verify: `go version` (should show 1.21+)
- Set `GOPATH` (default: `~/go`)
- Ensure `~/go/bin` is in `PATH`
- `GOCACHE` for build cache (default: `~/.cache/go-build`)

## Essential Go tools
```bash
go install golang.org/x/tools/gopls@latest      # LSP server
go install github.com/go-delve/delve@latest     # Debugger
go install github.com/golangci/golangci-lint@latest  # Linter
go install golang.org/x/tools/cmd/goimports@latest    # Import management
go install github.com/air-verse/air@latest           # Hot reload
go install golang.org/x/vuln/cmd/govulncheck@latest   # Vulnerability scanner
```

## Editor configuration
### VS Code
- Install Go extension
- `gopls` for language server
- `golangci-lint` for linting
- Format on save: `gofmt` or `goimports`
- Debug: Delve integration

### GoLand (JetBrains)
- Built-in Go support
- Delve debugger included
- `golangci-lint` integration via plugin

## Project setup
```bash
# Initialize module
go mod init github.com/user/project

# Add dependencies
go get github.com/spf13/cobra

# Tidy dependencies
go mod tidy

# Vendor (optional, for reproducible builds)
go mod vendor
```

## Makefile essentials
```makefile
.PHONY: build test lint fmt
build:
	go build -o bin/app ./cmd/app
test:
	go test -race ./...
lint:
	golangci-lint run
fmt:
	gofmt -w .
	goimports -w .
```

## Development workflow
- `go run ./cmd/app` for quick runs
- `air` for hot reload during development
- `go test -v ./...` for verbose test output
- `go test -race ./...` for race detection
- `dlv debug` for debugging
- `go tool pprof` for profiling

## Common issues
- **GOPATH/bin not in PATH**: add `export PATH=$PATH:$(go env GOPATH)/bin`
- **module cache**: `go clean -modcache` to clear
- **build cache**: `go clean -cache` to clear
- **proxy issues**: `GOPROXY=https://goproxy.cn,direct` for China
- **CGO errors**: `CGO_ENABLED=0` for pure Go, or install GCC
