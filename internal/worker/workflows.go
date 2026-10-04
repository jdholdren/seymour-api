package worker

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/jdholdren/seymour/internal/seymour"
)

// NOTE: The workflow functions are really just methods hanging off of workflows for namespace
// organization. I'd like to not have the receivar var in there since it's not used, but Zed's
// outline feature doesn't detect the methods unless it's present, e.g. (w workflows) not (workflows)

type workflows struct{}

func (w workflows) SyncAllFeeds(ctx workflow.Context) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	l := workflow.GetLogger(ctx)

	var allFeedCount int
	if err := workflow.ExecuteActivity(ctx, acts.CountAllFeeds).Get(ctx, &allFeedCount); err != nil {
		l.Error("failed to sync all feeds", "error", err)
		return err
	}

	// Batch each one into a group to sync
	batches := allFeedCount / 50
	if batches*50 < allFeedCount {
		batches += 1
	}

	wg := workflow.NewWaitGroup(ctx)
	for i := range batches {
		// Get a page of feed ID's
		var ids []string
		if err := workflow.ExecuteActivity(ctx, acts.FeedIDPage, i*50, 50).Get(ctx, &ids); err != nil {
			l.Error("failed to get feed IDs", "error", err)
			return err
		}
		wg.Add(len(ids))

		for _, id := range ids {
			workflow.Go(ctx, func(ctx workflow.Context) {
				defer wg.Done()

				if err := workflow.ExecuteActivity(ctx, acts.SyncFeed, id, true).Get(ctx, nil); err != nil {
					l.Error("failed to sync feed", "feed_id", id, "error", err)
				}
			})
		}

	}
	wg.Wait(ctx)

	return nil
}

// SubscribeToFeed triggers a workflow that creates the feed and subscription.
//
// Returns the new subscription's ID early, after the feed and subscription have
// been made, but before the feed is synced.
func SubscribeToFeed(ctx context.Context, c client.Client, feedURL, userID string) (string, error) {
	startOptions := c.NewWithStartWorkflowOperation(
		client.StartWorkflowOptions{
			TaskQueue: TaskQueue,
			// Required by UpdateWithStartWorkflow. No WorkflowID is set above, so
			// each call gets an auto-generated one and a collision isn't expected;
			// USE_EXISTING just means if one ever does occur, the update attaches
			// to the already-running workflow instead of failing the request.
			WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		},
		workflows{}.CreateFeed,
		createFeedArgs{FeedUrl: feedURL, UserID: userID},
	)
	updateHandle, err := c.UpdateWithStartWorkflow(
		ctx,
		client.UpdateWithStartWorkflowOptions{
			StartWorkflowOperation: startOptions,
			UpdateOptions: client.UpdateWorkflowOptions{
				UpdateName:   createFeedUpdateName,
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("unable to execute workflow: %s", err)
	}

	var subscriptionID string
	err = updateHandle.Get(context.Background(), &subscriptionID)
	seyErr := &seymour.Error{}
	if asSeyerr(err, &seyErr) {
		return "", seyErr
	}
	if err != nil {
		return "", fmt.Errorf("error executing workflow: %s", err)
	}

	return subscriptionID, nil
}

const createFeedUpdateName = "create_feed_update_creation"

type createFeedArgs struct {
	FeedUrl string
	UserID  string
}

// CreateFeed inserts a new feed, tries to sync, and rolls back if it's unable to.
// Once synced, it subscribes args.UserID to the feed.
//
// Returns the ID of the created subscription.
func (w workflows) CreateFeed(ctx workflow.Context, args createFeedArgs) (feedID string, err error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	l := workflow.GetLogger(ctx)
	ctx = workflow.WithActivityOptions(ctx, options)

	var (
		subscriptionID string
		setupDone      bool
		setupErr       error
	)
	if err := workflow.SetUpdateHandler(ctx, createFeedUpdateName,
		func(ctx workflow.Context) (string, error) {
			if err := workflow.Await(ctx, func() bool { return setupDone }); err != nil {
				return "", err
			}
			return subscriptionID, setupErr
		}); err != nil {
		return "", fmt.Errorf("error setting update handler: %s", err)
	}
	// The update handler above blocks until setupDone is true, however this
	// function returns, so make sure it's always eventually set: on any
	// early return (feed creation, sync, or subscription creation failing),
	// this reports the same error back to the caller of SubscribeToFeed
	// instead of leaving it hanging.
	defer func() {
		if !setupDone {
			setupErr = err
			setupDone = true
		}
	}()

	err = workflow.ExecuteActivity(ctx, acts.CreateFeed, args.FeedUrl).Get(ctx, &feedID)
	if err != nil {
		l.Error("failed to create feed", "error", err)
		return "", err
	}

	// Sync the feed before subscribing the user to it, so a subscription is
	// never created for a feed that couldn't actually be synced.
	if syncErr := workflow.ExecuteActivity(ctx, acts.SyncFeed, feedID).Get(ctx, nil); syncErr != nil {
		l.Error("failed to sync feed", "feed_id", feedID, "error", syncErr)

		// If there's an issue syncing, remove the feed
		if rmErr := workflow.ExecuteActivity(ctx, acts.RemoveFeed, feedID).Get(ctx, nil); rmErr != nil {
			l.Error("failed to remove feed", "feed_id", feedID, "error", rmErr)
			return "", rmErr
		}

		return "", syncErr
	}

	setupErr = workflow.ExecuteActivity(ctx, acts.CreateSubscription, args.UserID, feedID).Get(ctx, &subscriptionID)
	if setupErr != nil {
		l.Error("failed to create subscription", "error", setupErr)
		return "", setupErr
	}
	setupDone = true // Signal update handler with the real subscription ID

	// Wait for timeline materialization. Judgement runs independently.
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		TaskQueue: TaskQueue,
	})
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.RefreshTimeline).Get(ctx, nil); err != nil {
		l.Error("child workflow failed", "error", err)
		return "", err
	}

	return subscriptionID, nil
}

// RefreshTimeline materializes missing entries based on subscriptions.
func (w workflows) RefreshTimeline(ctx workflow.Context) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	l := workflow.GetLogger(ctx)

	if err := workflow.ExecuteActivity(ctx, acts.InsertMissingTimelineEntries).Get(ctx, nil); err != nil {
		l.Error("failed to insert missing timeline entries", "error", err)
		return err
	}

	return nil
}
