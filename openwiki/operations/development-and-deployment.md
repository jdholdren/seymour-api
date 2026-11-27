---
type: operations
title: Development and deployment
description: Documents local Docker Compose operation, required runtime configuration, startup/migration order, and CI build and validation workflows.
tags: [operations, docker, configuration, ci]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-f4927fdbd3f1f0d083362c6d
    resource: repo://.github/workflows/docker-publish.yml
  - id: openwiki-source-0b211c8d2e65f1265adb0432
    resource: repo://cmd/api/main.go
  - id: openwiki-source-b8a927b0cc8a1ee64f5a4f78
    resource: repo://docker-compose.yaml
  - id: openwiki-source-f422286848f713c7c7be771e
    resource: repo://internal/migrations/migrations.go
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Development and deployment

## Local services

Docker Compose runs the API and MySQL. MySQL uses a persistent named volume and a health check; the API waits for MySQL health before starting. The API is exposed on port 4444, while the database port is bound to localhost. `make up-d` starts services detached, `make rb-api` rebuilds and recreates only the API, and `make build` builds the Compose images (`docker-compose.yaml`, `Makefile`).

The Compose file supplies development-facing port, CORS, frontend, and OAuth callback values, and loads secrets from `.env`. Configure the GitHub OAuth client and session keys in that environment; avoid treating the checked-in development defaults as production secrets (`docker-compose.yaml`, `cmd/api/main.go`).

## Runtime configuration and startup

`cmd/api` requires `DATABASE`, `FRONTEND_URL`, GitHub client ID/secret and redirect URL, and hex-encoded session hash/block keys. `PORT` defaults to 4444 and `CORS` controls the allowed origin. The configured session hash key must decode to 32 or 64 bytes and the block key to 16, 24, or 32 bytes for securecookie (`cmd/api/main.go`, `internal/api/api.go`).

Startup processes configuration, initializes logging, opens MySQL, and applies embedded migrations before creating the repository or HTTP server. The server runs under a signal-aware run group and attempts graceful shutdown with a ten-second timeout (`cmd/api/main.go`, `internal/migrations/migrations.go`).

The Dockerfile builds a statically linked Linux Go binary and copies it with CA certificates into a minimal `scratch` image. Feed and OAuth network calls therefore rely on the included CA bundle, and `MAIN_PATH` selects the Go main package to build (`Dockerfile`).

## Validation and release automation

`make test` runs `go test ./...`; the test workflow invokes it on pushes and pull requests targeting `main`. The lint workflow runs golangci-lint on pushes. The release workflow publishes tagged images to GHCR using the Dockerfile's `MAIN_PATH` build argument (`Makefile`, `.github/workflows/test.yml`, `.github/workflows/lint.yml`, `.github/workflows/docker-publish.yml`).

The image publishing matrix currently includes both `./cmd/api` and `./cmd/worker`, while this repository's `cmd` tree currently contains only `cmd/api`. Treat the worker image entry as a mismatch to resolve before relying on that release matrix for a tagged build (`.github/workflows/docker-publish.yml`, `cmd/api/main.go`).

## Related topics

- [Quickstart](../quickstart.md) gives the task-routing map and common commands.
- [Testing strategy](../testing.md) describes the test layers and representative coverage.
- [Authentication and account lifecycle](../workflows/authentication.md) covers OAuth and cookie configuration.
