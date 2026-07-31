# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o /out/pcb ./cmd/pcb

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git docker-cli && \
    addgroup -S pcb && adduser -S -G pcb pcb
WORKDIR /app
COPY --from=build /out/pcb /usr/local/bin/pcb
ENV PATH=/usr/local/bin:/app
ENV PCB_CWD=/app/workspace
USER pcb
WORKDIR /app/workspace
ENTRYPOINT ["/usr/local/bin/pcb"]
