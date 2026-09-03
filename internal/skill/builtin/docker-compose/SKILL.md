---
name: docker-compose
description: Docker and docker-compose best practices for Go applications: multi-stage builds, layer caching, service composition, health checks, and volume management. Use when containerizing Go apps or managing multi-service development environments.
origin: builtin
---

# Docker & Compose

## When to use
- Containerizing a Go application
- Setting up multi-service development environments
- Optimizing Docker build performance
- Writing docker-compose.yml for dev/staging

## Multi-stage builds (Go)
```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/bin/app ./cmd/app

# Runtime stage
FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/bin/app /app
ENTRYPOINT ["/app"]
```

Key points:
- `CGO_ENABLED=0` for static binaries
- Copy `go.mod`/`go.sum` before source for layer caching
- Use distroless or alpine for small images
- Don't copy source code to the runtime image

## Dockerfile best practices
- Use specific version tags (not `latest`)
- Order COPY commands from least to most frequently changing
- Use `.dockerignore` to exclude `bin/`, `.git`, `node_modules`
- Run as non-root user
- Add `HEALTHCHECK` instruction
- Keep layers minimal — combine RUN commands

## docker-compose.yml patterns

### Development environment
```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - .:/app
    environment:
      - DB_HOST=db
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: appdb
      POSTGRES_PASSWORD: dev
    volumes:
      - db-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  db-data:
```

### Key compose patterns
- `depends_on` with `condition: service_healthy` for startup ordering
- Named volumes for persistent data
- `env_file` for environment-specific config
- Override files: `docker-compose.yml` + `docker-compose.override.yml`
- Profiles for optional services: `profiles: ["debug"]`

## Image optimization
- Use `alpine` or `distroless` base images
- Strip debug info: `-ldflags="-s -w"`
- Use `docker scan` or `trivy` for vulnerability scanning
- Multi-stage builds reduce final image size

## Security
- Never bake secrets into images — use environment or secrets
- Run as non-root (add `USER` directive)
- Use read-only filesystem: `readonly: true` in compose
- Limit resources: `deploy.resources.limits`
- Scan images for known CVEs

## Layer caching
- `COPY go.mod go.sum` before `COPY .` to cache `go mod download`
- Use `--target=builder` for dev builds
- Use BuildKit cache mounts: `RUN --mount=type=cache,target=/root/.cache/go-build`
