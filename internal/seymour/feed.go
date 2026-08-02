package seymour

import "context"

// FeedService provides data operations for feeds and their entries.
type FeedService interface {
	Feed(ctx context.Context, id string) (Feed, error)
	Feeds(ctx context.Context, ids []string) ([]Feed, error)
	FeedByURL(ctx context.Context, url string) (Feed, error)
	InsertFeed(ctx context.Context, url string) (Feed, error)
	DeleteFeed(ctx context.Context, id string) error
	CountAllFeeds(ctx context.Context) (int, error)
	FeedIDs(ctx context.Context, offset, pageSize int) ([]string, error)
	Entry(ctx context.Context, id string) (FeedEntry, error)
	Entries(ctx context.Context, ids []string) ([]FeedEntry, error)
	InsertEntries(ctx context.Context, entries []FeedEntry) error
	UpdateFeed(ctx context.Context, id string, args UpdateFeedArgs) error
}

// Feed represents an RSS feed's details.
type Feed struct {
	ID           string  `db:"id"`
	Title        *string `db:"title"`
	URL          string  `db:"url"`
	Description  *string `db:"description"`
	LastSyncedAt *DBTime `db:"last_synced_at"`
	CreatedAt    DBTime  `db:"created_at"`
	UpdatedAt    DBTime  `db:"updated_at"`
}

// FeedEntry represents a unique entry in an RSS feed.
type FeedEntry struct {
	ID          string `db:"id"`
	FeedID      string `db:"feed_id"`
	GUID        string `db:"guid"`
	Title       string `db:"title"`
	Description string `db:"description"`
	CreatedAt   DBTime `db:"created_at"`
	PublishTime DBTime `db:"publish_time"`
	Link        string `db:"link"`
}

// UpdateFeedArgs holds the optional fields for updating a feed.
type UpdateFeedArgs struct {
	Title       string
	Description string
	LastSynced  DBTime
}
