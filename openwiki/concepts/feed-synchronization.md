---
type: concept
title: Feed synchronization and parsing
description: Describes how Seymour fetches RSS and Atom documents, normalizes entries, and stores shared feed data when a subscription is created.
tags: [feeds, rss, atom, synchronization]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:40:11.523Z
sources:
  - id: openwiki-source-e4e1ab9bb2b6b8f22574b318
    resource: repo://internal/api/timeline.go
  - id: openwiki-source-a0610e45f4981da31ddf8add
    resource: repo://internal/migrations/202501010_initial_schema.up.sql
  - id: openwiki-source-c968974d30a42b2dabf0a440
    resource: repo://internal/mysql/feeds.go
  - id: openwiki-source-2c3cf629f8020f64cea12912
    resource: repo://internal/sync/sync.go
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Feed synchronization and parsing

## Shared feed identity

Feed records are global rather than owned by a user. A unique URL identifies a feed, while each user's subscription is a separate record. When subscription creation encounters a feed URL conflict, the API reads the existing feed and synchronizes that feed before attaching the user subscription (`internal/api/timeline.go`, `internal/api/AGENTS.md`, `internal/mysql/feeds.go`).

## Fetch and format selection

`sync.Feed` performs an HTTP GET with a three-second client timeout, rejects non-200 responses, reads the body, and inspects the first XML start element to select Atom parsing when the root is `feed`; other or unrecognized documents take the RSS parsing path. Parsing and network failures are returned to the caller (`internal/sync/sync.go`).

RSS parsing maps channel metadata and items. Atom parsing maps feed title/subtitle and entries, preferring an `alternate` link when present and using entry content when summary is empty. Both parsers normalize IDs/GUIDs, links, title/description, and publication time into `seymour.Feed` and `FeedEntry` values. Text fields are sanitized to strip HTML, unescaped, trimmed, and bounded before storage (`internal/sync/sync.go`).

## Persistence and repeat syncs

The subscription handler inserts parsed entries through `FeedService.InsertEntries` and updates feed metadata and its last-synced time when both title and description are present. MySQL assigns IDs to new entries and uses the unique GUID constraint with `INSERT IGNORE`, so repeated synchronization does not create a duplicate GUID row (`internal/api/timeline.go`, `internal/mysql/feeds.go`, `internal/migrations/202501010_initial_schema.up.sql`).

The current subscription path performs fetch, parse, and entry persistence synchronously before creating the subscription. A failure in these steps aborts the request rather than leaving a successfully created subscription with an unsynced feed (`internal/api/timeline.go`).

## Boundaries and tests

The sync package owns parsing and normalization; it does not persist records. The API coordinates sync with feed and subscription services, and the MySQL repository owns durable insertion and uniqueness behavior. `internal/sync/sync_test.go` exercises feed parsing behavior; repository integration coverage for feed persistence is in `internal/mysql/feeds_test.go` (`internal/sync/sync.go`, `internal/api/timeline.go`).

## Related topics

- [Subscriptions and reader requests](../workflows/subscriptions-and-reading.md) describes the caller and reader path.
- [Domain contracts and MySQL persistence](../architecture/domain-and-persistence.md) covers storage interfaces and constraints.
- [Testing strategy](../testing.md) maps the focused tests.
