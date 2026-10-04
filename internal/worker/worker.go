package worker

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jdholdren/seymour/internal/seymour"
)

const TaskQueue = "shared"

// NewWorker registers workflows and activities and reconciles their schedules.
func NewWorker(ctx context.Context, feeds seymour.FeedService, timeline seymour.TimelineService, cli client.Client) (worker.Worker, error) {
	a := activities{feeds: feeds, timeline: timeline}
	w := worker.New(cli, TaskQueue, worker.Options{})
	wfs := workflows{}
	w.RegisterWorkflow(wfs.SyncAllFeeds)
	w.RegisterWorkflow(wfs.CreateFeed)
	w.RegisterWorkflow(wfs.RefreshTimeline)
	w.RegisterWorkflow(wfs.JudgeTimeline)
	w.RegisterActivity(&a)

	if err := ensureSchedules(ctx, cli); err != nil {
		return nil, fmt.Errorf("error ensuring schedules: %w", err)
	}
	return w, nil
}

// Temporal application error type, preserved across activity serialization.
const errTypeInternal = "internal"
