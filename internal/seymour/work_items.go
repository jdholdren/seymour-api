package seymour

import (
	"context"
	"encoding/json"
	"time"
)

// WorkType identifies the kind of work represented by a queue item.
type WorkType string

const (
	// WorkTypeFeedSync identifies work that synchronizes an RSS feed.
	WorkTypeFeedSync WorkType = "feed_sync"
)

// WorkQueueService provides operations for enqueueing and leasing work.
type WorkQueueService interface {
	// EnqueueWork creates work unless an incomplete item already has the same type and key.
	// A duplicate incomplete item returns an error with HTTP conflict status.
	EnqueueWork(ctx context.Context, workType WorkType, key string, input json.RawMessage) (WorkItem, error)

	// LeaseWork leases one available item matching the requested types.
	// An empty type list matches items of every type.
	// It returns ErrNotFound when no item is available.
	LeaseWork(ctx context.Context, workTypes []WorkType, leaseDuration time.Duration) (WorkItem, error)

	// CompleteWork marks an item complete only if the fencing generation matches
	// and the lease has not expired.
	CompleteWork(ctx context.Context, id string, fencingGeneration int64, result json.RawMessage) error

	// ReleaseExpiredWork clears expired leases and increments their generations,
	// making those items available for a new lease.
	ReleaseExpiredWork(ctx context.Context) (int64, error)
}

// WorkItem represents a unit of work and its current lease state.
type WorkItem struct {
	// ID uniquely identifies this work item.
	ID string `db:"id"`

	// Type identifies the kind of work this item represents.
	Type WorkType `db:"type"`

	// Key identifies this item uniquely among incomplete items of the same type.
	Key string `db:"key"`

	// Input contains the optional JSON payload supplied when the item was enqueued.
	Input *json.RawMessage `db:"input"`

	// Result contains the optional JSON payload recorded when the item was completed.
	Result *json.RawMessage `db:"result"`

	// LeasedAt records when the current lease was granted, or nil when unleased.
	LeasedAt *time.Time `db:"leased_at"`

	// LeasedUntil records when the current lease expires, or nil when unleased.
	LeasedUntil *time.Time `db:"leased_until"`

	// FencingGeneration invalidates updates from workers holding an older lease.
	FencingGeneration int64 `db:"fencing_generation"`

	// CompletedAt records when the item was completed, or nil while incomplete.
	CompletedAt *time.Time `db:"completed_at"`
}
