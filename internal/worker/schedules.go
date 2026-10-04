package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

func ensureSchedules(ctx context.Context, cli client.Client) error {
	wfs := workflows{}

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

	return ensureJudgementSchedule(ctx, cli.ScheduleClient())
}

const judgementScheduleID = "judge_timeline"

func ensureJudgementSchedule(ctx context.Context, schedules client.ScheduleClient) error {
	spec := client.ScheduleSpec{
		Intervals: []client.ScheduleIntervalSpec{{Every: time.Minute}},
	}
	action := &client.ScheduleWorkflowAction{
		ID:        judgementScheduleID,
		Workflow:  workflows{}.JudgeTimeline,
		TaskQueue: TaskQueue,
	}
	handle := schedules.GetHandle(ctx, judgementScheduleID)
	if _, err := handle.Describe(ctx); err != nil {
		var notFound *serviceerror.NotFound
		if !errors.As(err, &notFound) {
			return fmt.Errorf("describe judgement schedule: %w", err)
		}
		_, err = schedules.Create(ctx, client.ScheduleOptions{
			ID:      judgementScheduleID,
			Spec:    spec,
			Action:  action,
			Overlap: enums.SCHEDULE_OVERLAP_POLICY_SKIP,
		})
		// Another worker replica may have created it concurrently.
		if err != nil && !errors.Is(err, temporal.ErrScheduleAlreadyRunning) {
			return fmt.Errorf("create judgement schedule: %w", err)
		}
	}
	return handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			schedule := input.Description.Schedule
			schedule.Spec = &spec
			schedule.Action = action
			if schedule.Policy == nil {
				schedule.Policy = &client.SchedulePolicies{}
			}
			schedule.Policy.Overlap = enums.SCHEDULE_OVERLAP_POLICY_SKIP
			return &client.ScheduleUpdate{Schedule: &schedule}, nil
		},
	})
}
