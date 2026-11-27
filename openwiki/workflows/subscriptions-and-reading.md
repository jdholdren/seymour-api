---
type: workflow
title: Subscriptions and reader requests
description: Follows authenticated subscription creation, listing and deletion, plus shared feed-entry article extraction and response caching.
tags: [subscriptions, reader, api, authorization]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-12a4716d4c513001d6550754
    resource: repo://apis/v1/feedentries.go
  - id: openwiki-source-29d94e54a4f7bf073fd9d283
    resource: repo://apis/v1/subscriptions.go
  - id: openwiki-source-c0c2d534b0f007c7a8cdcafd
    resource: repo://internal/api/AGENTS.md
  - id: openwiki-source-e25cfa6b3c3cb5e2d305609f
    resource: repo://internal/api/api.go
  - id: openwiki-source-e4e1ab9bb2b6b8f22574b318
    resource: repo://internal/api/timeline.go
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Subscriptions and reader requests

## Subscription creation

`POST /api/users/{userID}/subscriptions` requires a session and first verifies that the path user matches the session identity. It validates that `feed_url` is present, inserts a globally shared feed or loads the existing record on URL conflict, synchronously fetches/parses the feed, and persists its entries. If both parsed title and description are available, it updates feed metadata and last-synced time. Only then does it create the user-feed subscription, read back subscription and feed state, and return a `SubscriptionResp` (`internal/api/timeline.go`, `apis/v1/subscriptions.go`).

This means remote feed retrieval is on the request path: parsing or persistence failures prevent the subscription record from being created. The database keeps feed identity shared, while the subscription associates it to an individual user (`internal/api/AGENTS.md`, `internal/migrations/202501010_initial_schema.up.sql`).

## Listing and deletion

`GET /api/users/{userID}/subscriptions` applies the same path-user check, loads that user's subscriptions, then fetches each referenced feed to populate response name, description, and last-synced metadata. The response is a list of `SubscriptionResp` values. `DELETE /api/subscriptions/{subscriptionID}` loads the subscription, compares its owner with the session user, and deletes only after authorization (`internal/api/timeline.go`, `apis/v1/subscriptions.go`).

## Reader content

`GET /api/feed-entries/{feedEntryID}` requires authentication but does not require ownership: feed entries belong to the shared catalog. It loads the entry and first checks the process-local LRU response cache. On a miss, it fetches the entry link with a two-second HTTP timeout, runs go-readability extraction, sanitizes the resulting HTML, constructs a `FeedEntryResp`, caches it by entry ID, and writes JSON (`internal/api/timeline.go`, `internal/api/api.go`, `apis/v1/feedentries.go`, `internal/api/AGENTS.md`).

The cache avoids repeated article fetch and extraction within a process, but it is not durable or shared across API instances. Entry lookup and article retrieval errors are returned through the API's common error adapter (`internal/api/api.go`, `internal/api/timeline.go`).

## Related topics

- [Authentication and account lifecycle](authentication.md) describes session identity and route protection.
- [Feed synchronization](../concepts/feed-synchronization.md) covers the feed parsing work triggered by creation.
- [Domain contracts and MySQL persistence](../architecture/domain-and-persistence.md) documents shared feeds and user-scoped subscription storage.
