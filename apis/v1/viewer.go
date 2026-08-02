package v1

// Viewer is the response of GET /api/viewer.
type Viewer struct {
	Subscriptions map[string]ViewerSubscription `json:"subscriptions"`
}

// ViewerSubscription describes a single subscription as seen by the viewer.
type ViewerSubscription struct {
	Name        string `json:"name"`
	FeedID      string `json:"feed_id"`
	Description string `json:"description"`
}
