package seymour

import (
	"context"
	"time"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
)

// TimelineService provides data operations for the curated timeline and
// the subscriptions that feed it.
type TimelineService interface {
	CreateSubscription(ctx context.Context, userID, feedID string) error
	AllSubscriptions(ctx context.Context, userID string) ([]Subscription, error)
	Subscription(ctx context.Context, id string) (Subscription, error)
	DeleteSubscription(ctx context.Context, id string) error
	MissingEntries(ctx context.Context) ([]MissingEntry, error)
	EntriesNeedingJudgement(ctx context.Context, limit uint) ([]TimelineEntry, error)
	InsertEntry(ctx context.Context, entry TimelineEntry) error
	UpdateTimelineEntry(ctx context.Context, id string, status TimelineEntryStatus) error
	TimelineEntries(ctx context.Context, args TimelineEntriesArgs) ([]TimelineEntry, error)
	CountTimelineEntries(ctx context.Context, args TimelineEntriesArgs) (int, error)

	// CreateFilter inserts a new filter into storage, returning its ID.
	CreateFilter(ctx context.Context, userID string, filter Filter) (string, error)
	// UserFilters fetches all filters belonging to a user.
	UserFilters(ctx context.Context, userID string) ([]Filter, error)
	// Filter fetches a single filter by ID.
	Filter(ctx context.Context, id string) (Filter, error)
	// DeleteFilter removes a filter, and its backing config, from storage.
	DeleteFilter(ctx context.Context, id string) error
}

// Subscription represents a subscription to a feed.
type Subscription struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	FeedID    string    `db:"feed_id"`
	CreatedAt time.Time `db:"created_at"`
}

// TimelineEntry represents an entry in the timeline.
type TimelineEntry struct {
	ID          string    `db:"id"`
	UserID      string    `db:"user_id"`
	FeedEntryID string    `db:"feed_entry_id"`
	CreatedAt   time.Time `db:"created_at"`
	FeedID      string    `db:"feed_id"`

	// For curation: whether the entry has been approved by the judge
	Status TimelineEntryStatus `db:"status"`
}

// MissingEntry is an instance where a feed entry should have been added to a user's timeline.
type MissingEntry struct {
	UserID      string `db:"user_id"`
	FeedEntryID string `db:"feed_entry_id"`
	FeedID      string `db:"feed_id"`
}

// TimelineEntriesArgs holds arguments for filtering timeline entries.
type TimelineEntriesArgs struct {
	UserID string              // Required: scope to a single user
	Status TimelineEntryStatus // To optionally filter by status
	FeedID string              // To optionally filter by feed
	Limit  uint64              // To optionally limit the number of entries returned

	// To optionally filter by the entry's feed publish date, inclusive on
	// both ends. The zero value means unset.
	FromDate apiv1.Date
	ToDate   apiv1.Date

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

// FilterType enumerates the different filter "features" a user can
// configure to help curate their timeline during judgement.
type FilterType string

const (
	FilterTypeAllowList    FilterType = "allow_list"
	FilterTypeDisallowList FilterType = "disallow_list"
)

// Filter is implemented by every filter config type, giving a bit of
// polymorphism since each config has a different shape.
type Filter interface {
	Type() FilterType
}

// DisallowListConfig rejects an entry outright if its title or description
// matches any of Keywords.
type DisallowListConfig struct {
	ID       string // ID of the underlying user_filters row.
	UserID   string // UserID of the underlying user_filters row.
	Keywords []string
}

func (DisallowListConfig) Type() FilterType { return FilterTypeDisallowList }

// AllowListConfig, when present, requires an entry's title or description
// to match one of Keywords in order to be approved.
type AllowListConfig struct {
	ID       string // ID of the underlying user_filters row.
	UserID   string // UserID of the underlying user_filters row.
	Keywords []string
}

func (AllowListConfig) Type() FilterType { return FilterTypeAllowList }
