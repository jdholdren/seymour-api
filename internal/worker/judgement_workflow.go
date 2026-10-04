package worker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const judgementBatchesPerRun = 100

// JudgeTimeline drains pending judgements. Each activity result is recorded by
// Temporal before persistence starts, so database retries reuse the decisions.
func (w workflows) JudgeTimeline(ctx workflow.Context) error {
	judgeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Minute,
			BackoffCoefficient:     2,
			MaximumAttempts:        3,
			NonRetryableErrorTypes: []string{errTypeInternal},
		},
	})
	persistCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Second,
		ScheduleToCloseTimeout: 24 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2,
			MaximumInterval:        time.Minute,
			NonRetryableErrorTypes: []string{errTypeInternal},
		},
	})

	for range judgementBatchesPerRun {
		var decisions judgements
		if err := workflow.ExecuteActivity(judgeCtx, acts.JudgeEntries).Get(ctx, &decisions); err != nil {
			return err
		}
		if len(decisions) == 0 {
			return nil
		}
		if err := workflow.ExecuteActivity(persistCtx, acts.MarkEntriesAsJudged, decisions).Get(ctx, nil); err != nil {
			return err
		}
	}
	return workflow.NewContinueAsNewError(ctx, w.JudgeTimeline)
}
