// Package v1 contains the wire types for the timeline API group, at
// version v1. See apis/subscriptions/v1 for the rationale on this layout.
package v1

import "time"

// PaginationMeta holds pagination metadata for API responses.
type PaginationMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total,omitempty"` // Optional total count
}

// TimelineEntry is a single item in a timeline response.
type TimelineEntry struct {
	EntryID     string    `json:"entry_id"`
	FeedName    string    `json:"feed_name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	PublishDate time.Time `json:"publish_date"`
}

// TimelineResp is the response of GET /api/timeline.
type TimelineResp struct {
	Items      []TimelineEntry `json:"items"`
	Pagination PaginationMeta  `json:"pagination"`
}
