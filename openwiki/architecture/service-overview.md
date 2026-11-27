---
type: architecture
title: Service architecture
description: Maps Seymour's Go API runtime, service boundaries, and the main request and persistence flows.
tags: [architecture, api, runtime]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-0b211c8d2e65f1265adb0432
    resource: repo://cmd/api/main.go
  - id: openwiki-source-e25cfa6b3c3cb5e2d305609f
    resource: repo://internal/api/api.go
  - id: openwiki-source-9adabd7be49fedf540a27a8c
    resource: repo://internal/api/auth.go
  - id: openwiki-source-e4e1ab9bb2b6b8f22574b318
    resource: repo://internal/api/timeline.go
  - id: openwiki-source-640c97eb8129985d000b9be3
    resource: repo://internal/seymour/filter.go
  - id: openwiki-source-9c1154d9ed450c262ca241b6
    resource: repo://internal/seymour/work_items.go
  - id: openwiki-source-2c3cf629f8020f64cea12912
    resource: repo://internal/sync/sync.go
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Service architecture

## Runtime composition

The executable is `cmd/api`. It loads environment configuration, configures structured logging, opens MySQL, applies embedded migrations, and creates one `internal/mysql.Repo`. That repository is supplied as the feed, subscription, and user services to `internal/api.NewServer`. The HTTP server runs in an `oklog/run` group with an interrupt handler and a bounded graceful-shutdown period (`cmd/api/main.go`).

`internal/api` owns HTTP routing, OAuth/session handling, request validation, and API response construction. `internal/seymour` defines domain models, service interfaces, and shared errors; `internal/mysql` supplies their MySQL implementation. `internal/sync` fetches and normalizes RSS/Atom feeds. `apis/v1` contains shared request and response shapes (`internal/api/api.go`, `internal/seymour/seymour.go`, `internal/mysql/mysql.go`).

## HTTP boundaries

`NewServer` creates a Gorilla Mux router, configures CORS and HTTP timeouts, and registers public viewer/OAuth/logout routes separately from protected API routes. The `/api` subrouter applies session authentication; handlers use the attached user ID for authorization. Error-returning handlers are adapted to `http.Handler` and encode `seymour.Error` statuses as JSON, converting unexpected errors to internal-server errors (`internal/api/api.go`, `internal/api/auth.go`).

The current server's protected routes cover subscription management and individual feed-entry reading. API handlers call service interfaces rather than issuing SQL directly, while the feed-entry reader additionally fetches article HTML, extracts readable content, sanitizes it, and caches the response (`internal/api/api.go`, `internal/api/timeline.go`).

## Representative data flows

On subscription creation, the handler checks that the requested user matches the authenticated session, inserts or reuses a globally shared feed record, synchronously fetches and parses it through `internal/sync`, persists entries and available feed metadata, then creates and reads back a user subscription for the response (`internal/api/timeline.go`, `internal/sync/sync.go`).

For account login, the public OAuth callback exchanges the provider response and calls the user service to ensure an account/login mapping exists. The server then issues a secure session cookie; later protected requests decode that cookie and receive the account ID in request context (`internal/api/oauth.go`, `internal/api/auth.go`, `internal/mysql/users.go`).

The domain/repository boundary also supports filters and a durable work queue, with their contracts and MySQL operations implemented even though they are not registered as HTTP routes in the current server. The queue stores typed work, leases records transactionally, and fences stale workers (`internal/seymour/filter.go`, `internal/seymour/work_items.go`, `internal/mysql/work_items.go`).

## State ownership

Feeds and feed entries are shared catalog state; subscriptions and filters are associated with a user. MySQL is the durable store, and migrations run as part of executable startup. The API also keeps an in-memory LRU of reader responses, so that cache is process-local and is not authoritative state (`internal/api/AGENTS.md`, `internal/api/api.go`, `internal/migrations/migrations.go`).

## Related topics

- [Domain contracts and MySQL persistence](domain-and-persistence.md) covers service interfaces and schema.
- [Authentication and account lifecycle](../workflows/authentication.md) traces OAuth and sessions.
- [Subscriptions and reader requests](../workflows/subscriptions-and-reading.md) follows API behavior.
- [Feed synchronization](../concepts/feed-synchronization.md) covers RSS/Atom normalization.
- [Work queue](../concepts/work-queue.md) explains persistent background-work primitives.
