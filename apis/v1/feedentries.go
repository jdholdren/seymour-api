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

// FeedEntrySummaryResp is an entry in the response of GET /api/feeds/{feedID}/entries.
type FeedEntrySummaryResp struct {
	ID          string    `json:"id"`
	FeedID      string    `json:"feed_id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	PublishTime time.Time `json:"publish_time"`
}

// FeedEntriesResp contains the entries belonging to a feed.
type FeedEntriesResp struct {
	Page    int                    `json:"page"`
	Size    int                    `json:"size"`
	Total   int                    `json:"total"`
	Entries []FeedEntrySummaryResp `json:"entries"`
}
