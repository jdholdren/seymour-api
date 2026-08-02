package v1

import "time"

// PostSubscriptionReq is the body of a POST /api/subscriptions request.
type PostSubscriptionReq struct {
	FeedURL string `json:"feed_url"`
}

// FeedResp describes a feed, returned as part of subscription responses.
type FeedResp struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Description  string     `json:"description"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// SubscriptionResp describes a single subscription.
type SubscriptionResp struct {
	ID              string     `json:"id"`
	FeedID          string     `json:"feed_id"`
	CreatedAt       time.Time  `json:"created_at"`
	FeedName        string     `json:"feed_name"`
	FeedDescription string     `json:"feed_description"`
	LastSynced      *time.Time `json:"last_synced"`
}

// SubscriptionListResp is the response of GET /api/subscriptions.
type SubscriptionListResp struct {
	Subscriptions []SubscriptionResp `json:"subscriptions"`
}
