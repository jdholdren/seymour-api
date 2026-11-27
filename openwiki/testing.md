---
type: testing
title: Testing strategy
description: Maps Seymour's unit and MySQL integration tests, shared test database lifecycle, and repository validation commands.
tags: [testing, go, mysql, ci]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-4f2678f93d3fd3835f9f2909
    resource: repo://.github/workflows/test.yml
  - id: openwiki-source-13a4ed25722682219de2e51b
    resource: repo://internal/mysql/feeds_test.go
  - id: openwiki-source-f3ce1865cb3ee514d58e8670
    resource: repo://internal/mysql/filters_test.go
  - id: openwiki-source-a7cfb8332df7892d25232933
    resource: repo://internal/mysql/mysql_test.go
  - id: openwiki-source-90aa15785bc939d2f6146ae4
    resource: repo://internal/mysql/work_items_test.go
  - id: openwiki-source-84d9f914b62f03e5129a3f5c
    resource: repo://internal/seymour/errors_test.go
  - id: openwiki-source-aaa88ada861ace59b061e10f
    resource: repo://internal/seymour/filter_test.go
  - id: openwiki-source-23308424c9691874b04e00fc
    resource: repo://internal/sync/sync_test.go
  - id: openwiki-source-012f2c78e3b1446dfc35803f
    resource: repo://Makefile
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Testing strategy

## Test layers

The `internal/seymour` package tests domain-level error construction and filter validation. `internal/sync` tests RSS and Atom parsing and format detection. `internal/mysql` tests persistence contracts against a real MySQL container, including feed deduplication, user creation, filter ownership/configuration, and work-queue lease fencing (`internal/seymour/*_test.go`, `internal/sync/sync_test.go`, `internal/mysql/*_test.go`).

The MySQL package's `TestMain` starts one MySQL 8.4 Testcontainers instance for the package, applies the real embedded migrations, and shares that database among tests. Each test gets a `Repo` and registers cleanup that truncates the tables, avoiding a separate container per test (`internal/mysql/mysql_test.go`).

## Behavior contracts covered

The persistence suite checks uniqueness and conflict behavior, user-scoped filter reads/updates/deletes, validation of stored filter configuration, and concurrent filter creation. Queue tests cover duplicate enqueue and re-enqueue after completion, lease selection, type filtering, expired lease release, stale fencing, and `SKIP LOCKED` behavior (`internal/mysql/feeds_test.go`, `internal/mysql/filters_test.go`, `internal/mysql/work_items_test.go`).

The sync tests use feed fixtures to check RSS/Atom normalization and XML-root format detection. Domain tests exercise filter one-of validation and structured error construction (`internal/sync/sync_test.go`, `internal/seymour/filter_test.go`, `internal/seymour/errors_test.go`).

## Running checks

Run the full Go test suite with `make test` (equivalent to `go test ./...`). Run an individual package with `go test ./internal/sync` or `go test ./internal/mysql`; MySQL integration tests require a working Docker/Testcontainers environment. CI's test workflow runs `make test`, while `make lint` and the lint workflow run golangci-lint (`Makefile`, `.github/workflows/test.yml`, `.github/workflows/lint.yml`).

The integration test DSN includes `parseTime=true` because the MySQL driver must decode date/time columns into `time.Time`; migrations are applied before the tests use the database (`internal/mysql/mysql_test.go`, `internal/mysql/AGENTS.md`).

## Related topics

- [Quickstart](quickstart.md) provides the common task-routing map.
- [Domain contracts and MySQL persistence](architecture/domain-and-persistence.md) documents database boundaries.
- [Feed synchronization](concepts/feed-synchronization.md) explains parser behavior.
- [Work queue](concepts/work-queue.md) covers lease invariants.
