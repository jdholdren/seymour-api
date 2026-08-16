package mysql_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
)

func TestInsertFeed_CreatesFeed(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	assert.NotEmpty(t, feed.ID)
	assert.Equal(t, "https://example.com/feed.xml", feed.URL)
}

func TestInsertFeed_ConflictOnDuplicateURL(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	_, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	_, err = repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusConflict, sErr.Status)
}

func TestFeed_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	_, err := repo.Feed(ctx, "does-not-exist")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestFeed_Found(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	created, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	found, err := repo.Feed(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.URL, found.URL)
}

func TestFeedByURL_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	_, err := repo.FeedByURL(ctx, "https://example.com/does-not-exist.xml")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestFeedByURL_Found(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	created, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	found, err := repo.FeedByURL(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestFeeds_MultipleIDs(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed1, err := repo.InsertFeed(ctx, "https://example.com/feed-1.xml")
	require.NoError(t, err)
	feed2, err := repo.InsertFeed(ctx, "https://example.com/feed-2.xml")
	require.NoError(t, err)
	_, err = repo.InsertFeed(ctx, "https://example.com/feed-3.xml")
	require.NoError(t, err)

	feeds, err := repo.Feeds(ctx, []string{feed1.ID, feed2.ID})
	require.NoError(t, err)
	require.Len(t, feeds, 2)

	empty, err := repo.Feeds(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestDeleteFeed(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	require.NoError(t, repo.DeleteFeed(ctx, feed.ID))

	_, err = repo.Feed(ctx, feed.ID)
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestCountAllFeeds(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	count, err := repo.CountAllFeeds(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	_, err = repo.InsertFeed(ctx, "https://example.com/feed-1.xml")
	require.NoError(t, err)
	_, err = repo.InsertFeed(ctx, "https://example.com/feed-2.xml")
	require.NoError(t, err)

	count, err = repo.CountAllFeeds(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestFeedIDs_Pagination(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	for i := range 5 {
		_, err := repo.InsertFeed(ctx, "https://example.com/feed-"+string(rune('a'+i))+".xml")
		require.NoError(t, err)
	}

	page1, err := repo.FeedIDs(ctx, 0, 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := repo.FeedIDs(ctx, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	page3, err := repo.FeedIDs(ctx, 4, 2)
	require.NoError(t, err)
	assert.Len(t, page3, 1)
}

func TestInsertEntries_DedupesByGUID(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	entry := seymour.FeedEntry{
		FeedID:      feed.ID,
		Title:       "title",
		Description: "description",
		GUID:        "duplicate-guid",
		Link:        "https://example.com/entry",
	}

	// Insert twice with the same GUID; InsertEntries assigns a fresh ID to
	// each slice it's given, so capture both IDs to confirm only the first
	// insert's row actually landed.
	first := []seymour.FeedEntry{entry}
	require.NoError(t, repo.InsertEntries(ctx, first))

	second := []seymour.FeedEntry{entry}
	require.NoError(t, repo.InsertEntries(ctx, second))

	found, err := repo.Entries(ctx, []string{first[0].ID})
	require.NoError(t, err)
	assert.Len(t, found, 1)

	skipped, err := repo.Entries(ctx, []string{second[0].ID})
	require.NoError(t, err)
	assert.Empty(t, skipped)
}

func TestEntry_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	_, err := repo.Entry(ctx, "does-not-exist")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestEntry_Found(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	entries := []seymour.FeedEntry{{
		FeedID:      feed.ID,
		Title:       "title",
		Description: "description",
		GUID:        "some-guid",
		Link:        "https://example.com/entry",
	}}
	require.NoError(t, repo.InsertEntries(ctx, entries))

	found, err := repo.Entry(ctx, entries[0].ID)
	require.NoError(t, err)
	assert.Equal(t, entries[0].GUID, found.GUID)
}

func TestEntries_MultipleIDs(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	entries := []seymour.FeedEntry{
		{FeedID: feed.ID, Title: "1", Description: "d", GUID: "guid-1", Link: "https://example.com/1"},
		{FeedID: feed.ID, Title: "2", Description: "d", GUID: "guid-2", Link: "https://example.com/2"},
	}
	require.NoError(t, repo.InsertEntries(ctx, entries))

	found, err := repo.Entries(ctx, []string{entries[0].ID, entries[1].ID})
	require.NoError(t, err)
	assert.Len(t, found, 2)
}

func TestUpdateFeed_PartialFields(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/feed.xml")
	require.NoError(t, err)

	require.NoError(t, repo.UpdateFeed(ctx, feed.ID, seymour.UpdateFeedArgs{Title: "New Title"}))

	updated, err := repo.Feed(ctx, feed.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.Title)
	assert.Equal(t, "New Title", *updated.Title)
	assert.Nil(t, updated.Description)

	require.NoError(t, repo.UpdateFeed(ctx, feed.ID, seymour.UpdateFeedArgs{Description: "New Description"}))

	updated, err = repo.Feed(ctx, feed.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.Description)
	assert.Equal(t, "New Description", *updated.Description)
	// Title should still be set from the prior update.
	require.NotNil(t, updated.Title)
	assert.Equal(t, "New Title", *updated.Title)
}
