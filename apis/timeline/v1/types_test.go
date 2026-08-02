package v1

import (
	"encoding/json"
	"testing"
	"time"
)

// TestTimelineRespMarshalJSON pins down the wire format of TimelineResp,
// since external callers depend on these JSON tags remaining stable.
func TestTimelineRespMarshalJSON(t *testing.T) {
	publishDate := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	resp := TimelineResp{
		Items: []TimelineEntry{
			{
				EntryID:     "entry-1",
				FeedName:    "Some Feed",
				Title:       "A Title",
				Description: "A Description",
				URL:         "https://example.com/a",
				PublishDate: publishDate,
			},
		},
		Pagination: PaginationMeta{
			Limit:  20,
			Offset: 0,
			Total:  1,
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %s", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal error: %s", err)
	}

	items, ok := got["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 item under \"items\", got %#v", got["items"])
	}
	item := items[0].(map[string]any)

	wantItemKeys := map[string]any{
		"entry_id":     "entry-1",
		"feed_name":    "Some Feed",
		"title":        "A Title",
		"description":  "A Description",
		"url":          "https://example.com/a",
		"publish_date": publishDate.Format(time.RFC3339Nano),
	}
	for k, want := range wantItemKeys {
		if got := item[k]; got != want {
			t.Errorf("item[%q] = %v, want %v", k, got, want)
		}
	}

	pagination, ok := got["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected \"pagination\" object, got %#v", got["pagination"])
	}
	if pagination["limit"] != float64(20) || pagination["offset"] != float64(0) || pagination["total"] != float64(1) {
		t.Errorf("unexpected pagination: %#v", pagination)
	}
}
