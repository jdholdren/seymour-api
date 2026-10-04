package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func judgementEnvironment(judge func(context.Context) (judgements, error), persist func(context.Context, judgements) error) *testsuite.TestWorkflowEnvironment {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(judge, activity.RegisterOptions{Name: "JudgeEntries"})
	env.RegisterActivityWithOptions(persist, activity.RegisterOptions{Name: "MarkEntriesAsJudged"})
	return env
}

func TestJudgeTimelineDrainsBacklog(t *testing.T) {
	remaining, saved := 85, 0
	env := judgementEnvironment(func(context.Context) (judgements, error) {
		batch := judgements{}
		for i := range min(remaining, judgeBatchSize) {
			batch[fmt.Sprint(saved+i)] = i%2 == 0
		}
		return batch, nil
	}, func(_ context.Context, batch judgements) error {
		remaining -= len(batch)
		saved += len(batch)
		return nil
	})
	env.ExecuteWorkflow(workflows{}.JudgeTimeline)
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 85, saved)
	require.Zero(t, remaining)
}

func TestJudgeTimelineEmpty(t *testing.T) {
	persistCalls := 0
	env := judgementEnvironment(func(context.Context) (judgements, error) {
		return nil, nil
	}, func(context.Context, judgements) error {
		persistCalls++
		return nil
	})
	env.ExecuteWorkflow(workflows{}.JudgeTimeline)
	require.NoError(t, env.GetWorkflowError())
	require.Zero(t, persistCalls)
}

func TestJudgeTimelineRetriesPersistenceWithoutRejudging(t *testing.T) {
	judgeCalls, persistCalls := 0, 0
	want := judgements{"approved": true, "rejected": false}
	var saved judgements
	env := judgementEnvironment(func(context.Context) (judgements, error) {
		judgeCalls++
		if judgeCalls == 1 {
			return want, nil
		}
		return nil, nil
	}, func(_ context.Context, batch judgements) error {
		persistCalls++
		if persistCalls < 3 {
			return errors.New("database unavailable")
		}
		saved = batch
		return nil
	})
	env.ExecuteWorkflow(workflows{}.JudgeTimeline)
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 2, judgeCalls) // One judgement plus the empty-backlog check.
	require.Equal(t, 3, persistCalls)
	require.Equal(t, want, saved)
}

func TestJudgeTimelineJudgingFailure(t *testing.T) {
	judgeCalls, persistCalls := 0, 0
	env := judgementEnvironment(func(context.Context) (judgements, error) {
		judgeCalls++
		return nil, errors.New("provider unavailable")
	}, func(context.Context, judgements) error {
		persistCalls++
		return nil
	})
	env.ExecuteWorkflow(workflows{}.JudgeTimeline)
	require.Error(t, env.GetWorkflowError())
	require.Equal(t, 3, judgeCalls)
	require.Zero(t, persistCalls)
}

func TestJudgeTimelinePersistenceFailureStopsDrain(t *testing.T) {
	judgeCalls := 0
	env := judgementEnvironment(func(context.Context) (judgements, error) {
		judgeCalls++
		return judgements{"entry": true}, nil
	}, func(context.Context, judgements) error {
		return temporal.NewNonRetryableApplicationError("invalid decision", errTypeInternal, nil)
	})
	env.ExecuteWorkflow(workflows{}.JudgeTimeline)
	require.Error(t, env.GetWorkflowError())
	require.Equal(t, 1, judgeCalls)
}

func TestJudgeTimelineContinuesAfterPersisting(t *testing.T) {
	judgeCalls, persistCalls := 0, 0
	env := judgementEnvironment(func(context.Context) (judgements, error) {
		if judgeCalls != persistCalls {
			return nil, temporal.NewNonRetryableApplicationError("previous batch not persisted", errTypeInternal, nil)
		}
		judgeCalls++
		return judgements{fmt.Sprint(judgeCalls): true}, nil
	}, func(context.Context, judgements) error {
		persistCalls++
		return nil
	})
	env.ExecuteWorkflow(workflows{}.JudgeTimeline)
	var continued *workflow.ContinueAsNewError
	require.ErrorAs(t, env.GetWorkflowError(), &continued)
	require.Equal(t, judgementBatchesPerRun, judgeCalls)
	require.Equal(t, judgeCalls, persistCalls)
}

func TestRefreshTimelineDoesNotStartJudgement(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnActivity(acts.InsertMissingTimelineEntries, mock.Anything).Return(85, nil).Once()
	// No child workflow is registered: attempting to start one fails this test.
	env.ExecuteWorkflow(workflows{}.RefreshTimeline)
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
