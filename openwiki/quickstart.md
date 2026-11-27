---
type: guide
title: Quickstart and topic map
description: Entry point for contributors to the Seymour API, with local commands and routes to architecture, workflows, operations, and tests.
tags: [quickstart, navigation, development]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-0b211c8d2e65f1265adb0432
    resource: repo://cmd/api/main.go
  - id: openwiki-source-b8a927b0cc8a1ee64f5a4f78
    resource: repo://docker-compose.yaml
  - id: openwiki-source-e25cfa6b3c3cb5e2d305609f
    resource: repo://internal/api/api.go
  - id: openwiki-source-640c97eb8129985d000b9be3
    resource: repo://internal/seymour/filter.go
  - id: openwiki-source-9c1154d9ed450c262ca241b6
    resource: repo://internal/seymour/work_items.go
  - id: openwiki-source-012f2c78e3b1446dfc35803f
    resource: repo://Makefile
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Quickstart and topic map

Seymour is a Go RSS/Atom aggregator API with GitHub OAuth, user subscriptions, shared feed storage, and reader-content retrieval. The API executable is `cmd/api`; Docker Compose runs it with MySQL (`cmd/api/main.go`, `docker-compose.yaml`).

## Common commands

- `make up-d` starts the API and database in the background.
- `make rb-api` rebuilds and restarts the API container.
- `make test` runs `go test ./...`.
- `make lint` runs golangci-lint.

The API listens on port 4444 in the Compose setup and expects OAuth/session configuration from `.env`. MySQL data persists in the Compose volume (`Makefile`, `docker-compose.yaml`, `cmd/api/main.go`).

## Where to look

- For layer boundaries and request flow, start with [Service architecture](architecture/service-overview.md).
- For database/domain changes, read [Domain contracts and MySQL persistence](architecture/domain-and-persistence.md).
- For OAuth or cookie behavior, see [Authentication and account lifecycle](workflows/authentication.md).
- For subscribe/list/delete and article retrieval, see [Subscriptions and reader requests](workflows/subscriptions-and-reading.md).
- For RSS/Atom parsing, see [Feed synchronization](concepts/feed-synchronization.md).
- For durable queue leases and fencing, see [Work queue](concepts/work-queue.md).
- For local configuration and release behavior, see [Development and deployment](operations/development-and-deployment.md).
- For test layout and execution, see [Testing strategy](testing.md).

## Important boundaries

Feeds and entries are shared across users; subscriptions and filters are user-scoped. The currently registered HTTP API includes OAuth/viewer, subscription management, and feed-entry reading. Filter and work-queue storage contracts exist, but are not currently exposed as HTTP routes (`internal/api/AGENTS.md`, `internal/api/api.go`, `internal/seymour/filter.go`, `internal/seymour/work_items.go`).
