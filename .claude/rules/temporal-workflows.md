---
paths:
  - "internal/worker/**"
  - "cmd/worker/**"
---

# Temporal workflows

- Task queue name: `shared`
- Singleton workflows use `WorkflowIDReusePolicy: TERMINATE_IF_RUNNING`
- Child workflows use `ParentClosePolicy: ABANDON` so they outlive parents
- Schedules: `sync_all` and `refresh_timelines` both run every 15 minutes

## Workflows

- `SyncAllFeeds` — batches feeds in groups of 50
- `CreateFeed` — creates feed, syncs, rolls back on failure
- `RefreshTimeline` — inserts missing timeline entries, triggers judging
- `JudgeTimeline` — approves/rejects entries via `JudgeEntries` (batches of
  `judgeBatchSize`, max 3 loops)

## The judging seam

`activities.JudgeEntries` in `judge.go` currently approves every entry.
It's the intended seam for a real curation strategy — the surrounding
workflow, batching, and persistence already exist, so a real implementation
only needs to replace that function's body.

## Env vars

`DATABASE` (MySQL DSN), `TEMPORAL_HOST_PORT`
