---
type: architecture
title: Domain contracts and MySQL persistence
description: Explains how Seymour's domain interfaces separate API behavior from MySQL storage, and how models, errors, and embedded migrations shape persistence contracts.
tags: [domain, persistence, mysql, migrations]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:40:11.523Z
sources:
  - id: openwiki-source-0b211c8d2e65f1265adb0432
    resource: repo://cmd/api/main.go
  - id: openwiki-source-a0610e45f4981da31ddf8add
    resource: repo://internal/migrations/202501010_initial_schema.up.sql
  - id: openwiki-source-1c63588ef796ef86d413ea9d
    resource: repo://internal/migrations/20261127000000_work_items.up.sql
  - id: openwiki-source-f422286848f713c7c7be771e
    resource: repo://internal/migrations/migrations.go
  - id: openwiki-source-135809e10a360faab3ec6dbf
    resource: repo://internal/mysql/filters.go
  - id: openwiki-source-9751ad54929f9a7f9d20fb18
    resource: repo://internal/mysql/mysql.go
  - id: openwiki-source-14667e4fcf2b7e2fae71b0b8
    resource: repo://internal/mysql/users.go
  - id: openwiki-source-e38b27e6237cd666116fe6ea
    resource: repo://internal/seymour/errors.go
  - id: openwiki-source-640c97eb8129985d000b9be3
    resource: repo://internal/seymour/filter.go
  - id: openwiki-source-b2872c1f26c042f90c02d328
    resource: repo://internal/seymour/seymour.go
generated: { by: "opencode", at: "2026-11-27T18:40:11.523Z" }
---

# Domain contracts and MySQL persistence

## Ownership boundary

`internal/seymour` owns shared domain models and service interfaces. The API depends on these interfaces rather than SQL details, while `internal/mysql.Repo` implements the user, feed, subscription, filter, and work-queue services. The executable constructs one repository from `sqlx.DB` and passes it into the server for its service dependencies (`internal/seymour/seymour.go`, `internal/mysql/mysql.go`, `cmd/api/main.go`).

The models are also the database mapping boundary: fields commonly carry `db` tags and use `time.Time` values, with nullable database values represented by pointers where appropriate. A DSN with `parseTime=true` is required for MySQL timestamp decoding (`internal/seymour/feed.go`, `internal/seymour/user.go`, `internal/mysql/AGENTS.md`).

## Domain behavior and errors

Service interfaces describe operations in terms of domain entities and context, leaving SQL execution and row mapping to the repository. For example, `FeedService` owns feed and entry lookup/insertion/update operations; `SubscriptionService` owns user-feed relationships; and `UserService.EnsureUser` gives OAuth account creation a persistence-level idempotent operation (`internal/seymour/feed.go`, `internal/seymour/timeline.go`, `internal/seymour/user.go`).

`seymour.Error` carries an HTTP status and optional field details across service and handler boundaries. Its JSON representation exposes message, details, and status, while sentinels represent not-found and conflict outcomes. This lets API error handling preserve expected client statuses rather than flattening domain failures into generic server errors (`internal/seymour/errors.go`, `internal/api/api.go`).

Filters illustrate domain validation independent of storage: a filter must have exactly one configuration matching its type, and its selected configuration is validated even when disabled. The MySQL repository stores the selected configuration as JSON, reconstructs the typed domain variant when reading, and validates stored data before returning it (`internal/seymour/filter.go`, `internal/mysql/filters.go`).

## MySQL repository behavior

`Repo` wraps `sqlx.DB` and implements the service interfaces. The implementation uses parameterized SQL and Squirrel for queries that are assembled dynamically. IDs are UUIDs with entity-specific suffixes; uniqueness constraints arbitrate concurrent creates, with known duplicate-key outcomes mapped to conflicts where the service contract calls for it (`internal/mysql/mysql.go`, `internal/mysql/feeds.go`, `internal/mysql/filters.go`, `internal/mysql/users.go`).

Feeds and entries form a shared catalog, while subscriptions and filters are user-scoped. Feed URLs are unique, and feed-entry GUIDs are unique; insertion skips already-seen entries so repeated feed retrieval does not duplicate them. Subscription listing and filter queries scope rows by user ID, and filter lookup/update/delete additionally enforce ownership in the repository (`internal/migrations/202501010_initial_schema.up.sql`, `internal/mysql/feeds.go`, `internal/mysql/timeline.go`, `internal/mysql/filters.go`).

Account creation is transactional: `EnsureUser` first looks for the identity-provider login, then creates a user and login together. If a competing request wins the unique identity-provider constraint, it rolls back the partial attempt and reads the winner's records (`internal/mysql/users.go`).

## Schema lifecycle

SQL migration files live in `internal/migrations` and are embedded into the Go binary. Startup parses the configured MySQL DSN, creates a `golang-migrate` MySQL driver using its explicit database name, and applies all pending migrations before constructing the repository and serving requests (`internal/migrations/migrations.go`, `cmd/api/main.go`).

The initial schema defines feeds, feed entries, users, identity-provider logins, subscriptions, and filters. Feed URLs, entry GUIDs, user-feed subscriptions, provider identities, and each user's filter type have uniqueness constraints. Filter rows store a JSON object with checks for supported types and boolean enabled state; the domain layer additionally validates the type-specific configuration (`internal/migrations/202501010_initial_schema.up.sql`, `internal/seymour/filter.go`, `internal/mysql/filters.go`).

The work-items migration adds durable queue records, nullable lease and completion timestamps, JSON input/result, a fencing generation, and an active-item uniqueness key that permits completed history while preventing duplicate incomplete work of the same type and key (`internal/migrations/20261127000000_work_items.up.sql`).

Migration rollback files reverse each migration. Since startup applies migrations automatically, a schema change must be represented by an ordered up/down SQL pair and reviewed together with the repository queries and domain contracts that depend on it (`internal/migrations`, `internal/migrations/migrations.go`).

## Related topics

- [Service architecture](service-overview.md) explains runtime wiring and cross-layer request flow.
- [Work queue and lease fencing](../concepts/work-queue.md) details queue persistence and concurrency guarantees.
- [Testing strategy](../testing.md) maps repository integration tests and their database setup.
