---
type: concept
title: Work queue and lease fencing
description: Explains durable work-item identity, MySQL leasing and expiration, and how fencing generations prevent stale workers from completing work.
tags: [work-queue, leases, mysql, concurrency]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-0b211c8d2e65f1265adb0432
    resource: repo://cmd/api/main.go
  - id: openwiki-source-e25cfa6b3c3cb5e2d305609f
    resource: repo://internal/api/api.go
  - id: openwiki-source-1c63588ef796ef86d413ea9d
    resource: repo://internal/migrations/20261127000000_work_items.up.sql
  - id: openwiki-source-9751ad54929f9a7f9d20fb18
    resource: repo://internal/mysql/mysql.go
  - id: openwiki-source-201d813be07b77456806e928
    resource: repo://internal/mysql/work_items.go
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Work queue and lease fencing

## Domain contract

`WorkQueueService` exposes enqueue, lease, complete, and expired-lease release operations. A work item records a type and key, optional JSON input/result, lease timestamps, completion time, and a fencing generation. Empty type filters lease any type; no available item returns `ErrNotFound` (`internal/seymour/work_items.go`).

## Durable identity and enqueue

`EnqueueWork` rejects malformed nonempty JSON input and inserts a new UUID-based work ID. A unique database key permits only one incomplete item for a given type/key; completed rows remain as history and do not block that key from recurring. Duplicate active items become a conflict error (`internal/mysql/work_items.go`, `internal/migrations/20261127000000_work_items.up.sql`).

## Leasing and concurrency

Leasing requires a positive duration and runs in a transaction. It selects the earliest eligible row by ID, requiring it to be incomplete and either unleased or expired. `FOR UPDATE SKIP LOCKED` lets concurrent workers avoid blocking on a row already selected by another transaction. An optional type list narrows the selection; an empty list deliberately leaves it unrestricted (`internal/mysql/work_items.go`).

The initial lease uses the row's existing generation (default 1). Re-leasing an expired lease increments the generation. This generation is the fencing token supplied back during completion, so a worker holding an older lease cannot commit after another lease has taken ownership (`internal/mysql/work_items.go`, `internal/migrations/20261127000000_work_items.up.sql`).

## Completion and recovery

Completion accepts optional valid JSON result and updates only an incomplete row whose generation matches and whose lease has not expired. A zero-row update is a conflict, covering stale generations, expired leases, missing items, and already completed work. `ReleaseExpiredWork` clears expired lease timestamps and increments generations, making the rows available while invalidating prior workers (`internal/mysql/work_items.go`).

## Operational boundary

The durable schema and repository contract exist, but the current HTTP router does not expose queue operations and the executable does not start a worker loop. Queue consumers therefore need to be supplied by another runtime/integration; this package currently provides the persistence primitives rather than scheduling or executing work (`internal/api/api.go`, `cmd/api/main.go`, `internal/mysql/mysql.go`).

## Related topics

- [Domain contracts and MySQL persistence](../architecture/domain-and-persistence.md) covers repositories and migrations.
- [Service architecture](../architecture/service-overview.md) maps runtime composition.
- [Testing strategy](../testing.md) covers queue integration tests.
