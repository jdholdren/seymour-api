package seymour

import "context"

// TimelineService provides data operations for the curated timeline and
// the subscriptions that feed it.
type TimelineService interface {
	CreateSubscription(ctx context.Context, feedID string) error
	AllSubscriptions(ctx context.Context) ([]Subscription, error)
	MissingEntries(ctx context.Context) ([]MissingEntry, error)
	EntriesNeedingJudgement(ctx context.Context, limit uint) ([]TimelineEntry, error)
	InsertEntry(ctx context.Context, entry TimelineEntry) error
	UpdateTimelineEntry(ctx context.Context, id string, status TimelineEntryStatus) error
	TimelineEntries(ctx context.Context, args TimelineEntriesArgs) ([]TimelineEntry, error)
	CountTimelineEntries(ctx context.Context, args TimelineEntriesArgs) (int, error)
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

	// For curation: whether the entry has been approved by the judge
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
