package seymour

import (
	"context"
	"time"
)

// SubscriptionService provides data operations for user feed subscriptions.
type SubscriptionService interface {
	CreateSubscription(ctx context.Context, userID, feedID string) (string, error)
	AllSubscriptions(ctx context.Context, userID string) ([]Subscription, error)
	Subscription(ctx context.Context, id string) (Subscription, error)
	DeleteSubscription(ctx context.Context, id string) error
}

// Subscription represents a subscription to a feed.
type Subscription struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	FeedID    string    `db:"feed_id"`
	CreatedAt time.Time `db:"created_at"`
}
