package worker

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/jdholdren/seymour/internal/seymour"
)

const TaskQueue = "shared"

// NewWorker sets up the worker with registration of workflows, activities, and schedules.
func NewWorker(ctx context.Context, repo seymour.Repository, cli client.Client, claudeClient *anthropic.Client) (worker.Worker, error) {
	a := activities{
		repo:         repo,
		claudeClient: claudeClient,
	}

	w := worker.New(cli, TaskQueue, worker.Options{})

	if err := registerEverything(ctx, w, a, cli); err != nil {
		return nil, fmt.Errorf("error registering workflows and activities: %T, %v", err, err)
	}

	return w, nil
}

func registerEverything(ctx context.Context, w worker.Worker, a activities, cli client.Client) error {
	// Workflows
	wfs := workflows{}
	w.RegisterWorkflow(wfs.SyncAllFeeds)
	w.RegisterWorkflow(wfs.CreateFeed)
	w.RegisterWorkflow(wfs.RefreshTimeline)
	w.RegisterWorkflow(wfs.JudgeTimeline)

	// Activities
	w.RegisterActivity(&a)

	// Schedules:
	// Sync RSS feeds
	handle := cli.ScheduleClient().GetHandle(ctx, "sync_all")
	if _, err := handle.Describe(ctx); err != nil {
		handle, err = cli.ScheduleClient().Create(ctx, client.ScheduleOptions{
			ID: "sync_all",
			Spec: client.ScheduleSpec{
				Intervals: []client.ScheduleIntervalSpec{{Every: 15 * time.Minute}},
			},
			Action: &client.ScheduleWorkflowAction{
				ID:        "sync_all",
				Workflow:  wfs.SyncAllFeeds,
				TaskQueue: TaskQueue,
			},
			TriggerImmediately: true,
		})
		if err != nil {
			return err
		}
	}
	if err := handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			return &client.ScheduleUpdate{
				Schedule: &input.Description.Schedule,
			}, nil
		},
	}); err != nil {
		return err
	}
	// Refresh timelines
	handle = cli.ScheduleClient().GetHandle(ctx, "refresh_timelines")
	if _, err := handle.Describe(ctx); err != nil {
		handle, err = cli.ScheduleClient().Create(ctx, client.ScheduleOptions{
			ID: "refresh_timelines",
			Spec: client.ScheduleSpec{
				Intervals: []client.ScheduleIntervalSpec{{Every: 15 * time.Minute}},
			},
			Action: &client.ScheduleWorkflowAction{
				ID:        "refresh_timelines",
				Workflow:  wfs.RefreshTimeline,
				TaskQueue: TaskQueue,
			},
		})
		if err != nil {
			return err
		}
	}
	if err := handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			return &client.ScheduleUpdate{
				Schedule: &input.Description.Schedule,
			}, nil
		},
	}); err != nil {
		return err
	}

	return nil
}

// Error types
//
// These are error types in the temporal sense, not the general "go" error types sense.
// They are used since between activities error types are marshaled and type information is lost.
const (
	errTypeInternal  = "internal"
	errTypeRateLimit = "rateLimit"
)
