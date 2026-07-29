package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS Feed</title>
    <description>A test RSS feed</description>
    <link>https://example.com</link>
    <item>
      <title>RSS Post One</title>
      <link>https://example.com/post-1</link>
      <guid>rss-guid-1</guid>
      <description>First RSS post description</description>
      <pubDate>Mon, 01 Jan 2024 12:00:00 GMT</pubDate>
    </item>
    <item>
      <title>RSS Post Two</title>
      <link>https://example.com/post-2</link>
      <guid>rss-guid-2</guid>
      <description>Second RSS post description</description>
      <pubDate>Tue, 02 Jan 2024 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

const testAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom Feed</title>
  <subtitle>A test Atom feed</subtitle>
  <link href="https://example.com" rel="alternate"/>
  <entry>
    <title>Atom Post One</title>
    <id>atom-id-1</id>
    <link href="https://example.com/atom-1" rel="alternate"/>
    <summary>First Atom post summary</summary>
    <updated>2024-01-01T12:00:00Z</updated>
  </entry>
  <entry>
    <title>Atom Post Two</title>
    <id>atom-id-2</id>
    <link href="https://example.com/atom-2" rel="alternate"/>
    <content>Second Atom post content body</content>
    <updated>2024-01-02T12:00:00Z</updated>
  </entry>
</feed>`

func TestFeed_RSS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(testRSSFeed))
	}))
	defer srv.Close()

	feed, entries, err := Feed(context.Background(), "feed-123", srv.URL)
	require.NoError(t, err)

	assert.Equal(t, "feed-123", feed.ID)
	assert.Equal(t, "Test RSS Feed", *feed.Title)
	assert.Equal(t, "A test RSS feed", *feed.Description)

	require.Len(t, entries, 2)

	assert.Equal(t, "RSS Post One", entries[0].Title)
	assert.Equal(t, "rss-guid-1", entries[0].GUID)
	assert.Equal(t, "https://example.com/post-1", entries[0].Link)
	assert.Equal(t, "First RSS post description", entries[0].Description)
	assert.Equal(t, "feed-123", entries[0].FeedID)
	assert.False(t, entries[0].PublishTime.Time.IsZero())

	assert.Equal(t, "RSS Post Two", entries[1].Title)
	assert.Equal(t, "rss-guid-2", entries[1].GUID)
}

func TestFeed_Atom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(testAtomFeed))
	}))
	defer srv.Close()

	feed, entries, err := Feed(context.Background(), "feed-456", srv.URL)
	require.NoError(t, err)

	assert.Equal(t, "feed-456", feed.ID)
	assert.Equal(t, "Test Atom Feed", *feed.Title)
	assert.Equal(t, "A test Atom feed", *feed.Description)

	require.Len(t, entries, 2)

	// First entry has a summary
	assert.Equal(t, "Atom Post One", entries[0].Title)
	assert.Equal(t, "atom-id-1", entries[0].GUID)
	assert.Equal(t, "https://example.com/atom-1", entries[0].Link)
	assert.Equal(t, "First Atom post summary", entries[0].Description)
	assert.Equal(t, "feed-456", entries[0].FeedID)
	assert.False(t, entries[0].PublishTime.Time.IsZero())

	// Second entry has content instead of summary
	assert.Equal(t, "Atom Post Two", entries[1].Title)
	assert.Equal(t, "atom-id-2", entries[1].GUID)
	assert.Equal(t, "https://example.com/atom-2", entries[1].Link)
	assert.Equal(t, "Second Atom post content body", entries[1].Description)
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "rss feed",
			input:    `<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`,
			expected: "rss",
		},
		{
			name:     "atom feed",
			input:    `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`,
			expected: "atom",
		},
		{
			name:     "empty input defaults to rss",
			input:    "",
			expected: "rss",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFormat([]byte(tt.input))
			assert.Equal(t, tt.expected, got)
		})
	}
}
