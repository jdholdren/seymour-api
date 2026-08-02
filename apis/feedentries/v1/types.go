// Package v1 contains the wire types for the feedentries API group, at
// version v1. See apis/subscriptions/v1 for the rationale on this layout.
package v1

import "time"

// FeedEntryResp is the response of GET /api/feed-entries/{feedEntryID}.
type FeedEntryResp struct {
	ID            string    `json:"id"`
	FeedID        string    `json:"feed_id"`
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	ReaderContent string    `json:"reader_content"`
}
