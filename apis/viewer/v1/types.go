// Package v1 contains the wire types for the viewer API group, at version
// v1. See apis/subscriptions/v1 for the rationale on this layout.
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
