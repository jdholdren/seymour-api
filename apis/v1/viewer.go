package v1

// Viewer is the response of GET /api/viewer.
type Viewer struct {
	User          *ViewerUser                   `json:"user,omitempty"`
	Subscriptions map[string]ViewerSubscription `json:"subscriptions"`
}

// ViewerUser describes the authenticated user, if any.
type ViewerUser struct {
	ID            string `json:"id"`
	PreferredName string `json:"preferred_name,omitempty"`
}

// ViewerSubscription describes a single subscription as seen by the viewer.
type ViewerSubscription struct {
	Name        string `json:"name"`
	FeedID      string `json:"feed_id"`
	Description string `json:"description"`
}
