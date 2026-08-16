package mysql

import (
	"context"

	"github.com/jdholdren/seymour/internal/seymour"
)

func (r Repo) Feed(ctx context.Context, id string) (seymour.Feed, error) {
	return seymour.Feed{}, ErrNotImplemented
}

func (r Repo) Feeds(ctx context.Context, ids []string) ([]seymour.Feed, error) {
	return nil, ErrNotImplemented
}

func (r Repo) FeedByURL(ctx context.Context, url string) (seymour.Feed, error) {
	return seymour.Feed{}, ErrNotImplemented
}

func (r Repo) InsertFeed(ctx context.Context, url string) (seymour.Feed, error) {
	return seymour.Feed{}, ErrNotImplemented
}

func (r Repo) DeleteFeed(ctx context.Context, id string) error {
	return ErrNotImplemented
}

func (r Repo) CountAllFeeds(ctx context.Context) (int, error) {
	return 0, ErrNotImplemented
}

func (r Repo) FeedIDs(ctx context.Context, offset, pageSize int) ([]string, error) {
	return nil, ErrNotImplemented
}

func (r Repo) Entry(ctx context.Context, id string) (seymour.FeedEntry, error) {
	return seymour.FeedEntry{}, ErrNotImplemented
}

func (r Repo) Entries(ctx context.Context, ids []string) ([]seymour.FeedEntry, error) {
	return nil, ErrNotImplemented
}

func (r Repo) InsertEntries(ctx context.Context, entries []seymour.FeedEntry) error {
	return ErrNotImplemented
}

func (r Repo) UpdateFeed(ctx context.Context, id string, args seymour.UpdateFeedArgs) error {
	return ErrNotImplemented
}
