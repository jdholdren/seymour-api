package seymour

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"
)

// Repository provides a unified interface for all data operations.
// It combines the functionality of FeedService, TimelineService, and UserService
// into a single interface to simplify dependency injection and testing.
type Repository interface {
	// Feed operations
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

	// Prompt operations
	ActivePrompt(ctx context.Context) (*Prompt, error)
	SetPrompt(ctx context.Context, content string) (Prompt, error)

	// Timeline operations
	CreateSubscription(ctx context.Context, feedID string) error
	AllSubscriptions(ctx context.Context) ([]Subscription, error)
	MissingEntries(ctx context.Context) ([]MissingEntry, error)
	EntriesNeedingJudgement(ctx context.Context, limit uint) ([]TimelineEntry, error)
	InsertEntry(ctx context.Context, entry TimelineEntry) error
	UpdateTimelineEntry(ctx context.Context, id string, status TimelineEntryStatus) error
	TimelineEntries(ctx context.Context, args TimelineEntriesArgs) ([]TimelineEntry, error)
	CountTimelineEntries(ctx context.Context, args TimelineEntriesArgs) (int, error)
}

// Prompt represents a curation prompt for AI-based feed judging.
type Prompt struct {
	ID        string `db:"id"`
	Content   string `db:"content"`
	Active    bool   `db:"active"`
	CreatedAt DBTime `db:"created_at"`
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

// Subscription represents a subscription to a feed.
type Subscription struct {
	ID        string `db:"id"`
	FeedID    string `db:"feed_id"`
	CreatedAt DBTime `db:"created_at"`
}

// TimelineEntry represents an entry in the timeline.
type TimelineEntry struct {
	ID          string `db:"id"`
	FeedEntryID string `db:"feed_entry_id"`
	CreatedAt   DBTime `db:"created_at"`
	FeedID      string `db:"feed_id"`

	// For curation: if the entry has been approved or not by the AI
	Status TimelineEntryStatus `db:"status"`
}

// MissingEntry is an instance where a feed entry should have been added to the timeline.
type MissingEntry struct {
	FeedEntryID string `db:"feed_entry_id"`
	FeedID      string `db:"feed_id"`
}

// TimelineEntriesArgs holds arguments for filtering timeline entries.
type TimelineEntriesArgs struct {
	Status TimelineEntryStatus // To optionally filter by status
	FeedID string              // To optionally filter by feed
	Limit  uint64              // To optionally limit the number of entries returned

	// Pagination fields
	Offset uint64 // Offset for pagination
}

// TimelineEntryStatus represents the status of a timeline entry.
type TimelineEntryStatus string

const (
	TimelineEntryStatusRequiresJudgement TimelineEntryStatus = "requires_judgement"
	TimelineEntryStatusApproved          TimelineEntryStatus = "approved"
	TimelineEntryStatusRejected          TimelineEntryStatus = "rejected"
)

// DBTime is a sqlite-acceptable implementation of a time that can be marshaled in and out of
// a sqlite db.
type DBTime struct {
	Time time.Time
}

// Value implements [driver.Valuer].
func (t DBTime) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}

	return t.Time.Format(time.RFC3339), nil
}

// Scan implements the [sql.Scanner] interface.
func (t *DBTime) Scan(value any) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		t.Time = v
	case string:
		// Try to parse in the correct format:
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return fmt.Errorf("error parsing time format: %s", v)
		}

		t.Time = parsed
	default:
		return fmt.Errorf("unsupported type for Time.Scan: %T", value)
	}

	return nil
}
