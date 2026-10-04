# Judgement workflow

`JudgeTimeline` drains timeline entries whose status is
`requires_judgement`. It is deliberately independent of feed synchronization
and timeline materialization: pending judgement is durable database work, not
a side effect that depends on a particular refresh execution remaining alive.

## Lifecycle

1. `JudgeEntries` selects and judges at most 20 pending entries.
2. An empty result ends the workflow successfully.
3. `MarkEntriesAsJudged` writes the decisions to MySQL.
4. Only after persistence succeeds does the workflow fetch another batch.
5. After 100 successfully persisted batches, the workflow uses Continue-As-New
   to bound its history, then continues draining in a new run.

Pending entries are selected oldest first by `(created_at, id)`. The repository
query also returns the owning `user_id` so judgement can become personalized.

The current judge is a placeholder that approves every entry. Replace the
implementation in `judgement_activities.go` when adding curation logic; keep
the workflow's batch and persistence boundaries explicit.

## Scheduling and overlap

Worker startup reconciles a Temporal schedule with ID `judge_timeline`. It
starts `JudgeTimeline` every 15 minutes on the `shared` task queue and uses the
`SKIP` overlap policy. A scheduled run that finds no work exits. Timeline
refreshes only insert missing entries; they do not start, cancel, or wait for a
judgement run. Thus entries created after a drainer has finished are picked up
by the next scheduled run (normally within about 15 minutes).

Keep one scheduled drainer. Do not start manual concurrent drainers: selection
does not claim or lock rows, so concurrent runs could judge the same entries.
If immediate processing or multiple consumers are needed later, add an explicit
claim/lease mechanism or a signal-based singleton design first.

## Retry and failure behavior

Judging and persistence are separate Temporal activities on purpose. Temporal
records a successful judging activity result in workflow history before the
workflow starts persistence. If a persistence attempt fails transiently,
Temporal retries persistence with the recorded decisions; it does not rerun
judging for that batch. The persistence activity itself is safe to retry after
a partial write because writing the same final status again is idempotent.

Judging has a two-minute start-to-close timeout and up to three attempts, with
one-minute initial retry interval and exponential backoff. Persistence has a
30-second start-to-close timeout and retries for up to 24 hours, with backoff
capped at one minute. Errors of Temporal type `internal` are configured as
non-retryable for both activities.

If judging ultimately fails, the workflow stops and the next scheduled run can
try the still-pending entries again. If persistence keeps failing beyond its
24-hour schedule-to-close limit, the workflow stops without fetching the next
batch; a later scheduled run recovers pending database rows. Its new workflow
history does not contain the previous run's LLM result, so it may call the LLM
again. Exactly-once external LLM calls are not guaranteed: a provider may have
completed a request just before an activity timed out or lost its response.
Provider idempotency or a durable judgement-result record can reduce that risk
when integrating a real model.

## Tests

Workflow tests cover draining beyond the former 60-entry limit, empty queues,
judging failure, persistence retries without rejudging within a run, and
Continue-As-New only after persisted batches. The MySQL repository test covers
bounded deterministic pending-entry selection and user ID scanning.
