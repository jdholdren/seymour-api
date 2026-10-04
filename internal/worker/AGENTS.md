# Temporal workflows

- Task queue name: `shared`
- Judgement has one scheduled drainer; overlap policy is `SKIP`, never terminate
  an active judgement to start another. Do not start additional manual drainers.
- Schedules: `sync_all`, `refresh_timelines`, and `judge_timeline` run every
  15 minutes. Definitions live in `schedules.go`.

## Workflows

- `SyncAllFeeds` — batches feeds in groups of 50
- `CreateFeed` — creates feed, syncs, rolls back on failure
- `RefreshTimeline` — inserts missing timeline entries independently of judging
- `JudgeTimeline` — drains pending entries in batches of `judgeBatchSize`,
  persisting each batch before fetching the next. Continues-As-New after 100
  persisted batches to bound history.

## The judging seam

`activities.JudgeEntries` in `judgement_activities.go` currently approves every entry.
It's the intended seam for a real curation strategy — the surrounding
workflow, batching, and persistence already exist, so a real implementation
can replace that function's body. Keep judging and persistence as separate
activities: Temporal records decisions so transient DB failures retry persistence
without repeating the judge. Persistence retries for up to 24 hours; if the
workflow ultimately fails, a later scheduled run may rejudge pending entries.

This change has no legacy workflow replay path. Finish existing `JudgeTimeline`
and `RefreshTimeline` executions on the old worker before deploying it.

## Env vars

`DATABASE` (MySQL DSN), `TEMPORAL_HOST_PORT`
