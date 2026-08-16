package mysql

import (
	"context"

	"github.com/jdholdren/seymour/internal/seymour"
)

func (r Repo) CreateSubscription(ctx context.Context, userID, feedID string) error {
	return ErrNotImplemented
}

func (r Repo) AllSubscriptions(ctx context.Context, userID string) ([]seymour.Subscription, error) {
	return nil, ErrNotImplemented
}

func (r Repo) Subscription(ctx context.Context, id string) (seymour.Subscription, error) {
	return seymour.Subscription{}, ErrNotImplemented
}

func (r Repo) DeleteSubscription(ctx context.Context, id string) error {
	return ErrNotImplemented
}

func (r Repo) MissingEntries(ctx context.Context) ([]seymour.MissingEntry, error) {
	return nil, ErrNotImplemented
}

func (r Repo) EntriesNeedingJudgement(ctx context.Context, limit uint) ([]seymour.TimelineEntry, error) {
	return nil, ErrNotImplemented
}

func (r Repo) InsertEntry(ctx context.Context, entry seymour.TimelineEntry) error {
	return ErrNotImplemented
}

func (r Repo) UpdateTimelineEntry(ctx context.Context, id string, status seymour.TimelineEntryStatus) error {
	return ErrNotImplemented
}

func (r Repo) TimelineEntries(ctx context.Context, args seymour.TimelineEntriesArgs) ([]seymour.TimelineEntry, error) {
	return nil, ErrNotImplemented
}

func (r Repo) CountTimelineEntries(ctx context.Context, args seymour.TimelineEntriesArgs) (int, error) {
	return 0, ErrNotImplemented
}
