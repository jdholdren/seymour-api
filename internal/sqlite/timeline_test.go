package sqlite_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
	"github.com/jdholdren/seymour/internal/sqlite"
)

func TestDeleteSubscription(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.CreateSubscription(ctx, "user-1", "feed-1"))

	subs, err := repo.AllSubscriptions(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)

	require.NoError(t, repo.DeleteSubscription(ctx, subs[0].ID))

	subs, err = repo.AllSubscriptions(ctx, "user-1")
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestAllSubscriptions_ScopedByUser(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.CreateSubscription(ctx, "user-1", "feed-1"))
	require.NoError(t, repo.CreateSubscription(ctx, "user-2", "feed-1"))

	subs, err := repo.AllSubscriptions(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "user-1", subs[0].UserID)
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	err := repo.DeleteSubscription(ctx, "does-not-exist")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

// seedTimelineEntry creates a feed, a feed entry published at publishTime,
// and a timeline entry pointing to it, returning the timeline entry's ID.
func seedTimelineEntry(t *testing.T, repo sqlite.Repo, userID, guid string, publishTime time.Time, status seymour.TimelineEntryStatus) string {
	t.Helper()
	ctx := context.Background()

	feed, err := repo.InsertFeed(ctx, "https://example.com/"+guid)
	require.NoError(t, err)

	entries := []seymour.FeedEntry{{
		FeedID:      feed.ID,
		Title:       "title",
		Description: "description",
		GUID:        guid,
		Link:        "https://example.com/" + guid,
		PublishTime: seymour.DBTime{Time: publishTime},
	}}
	require.NoError(t, repo.InsertEntries(ctx, entries))

	require.NoError(t, repo.InsertEntry(ctx, seymour.TimelineEntry{
		UserID:      userID,
		FeedEntryID: entries[0].ID,
		FeedID:      feed.ID,
		Status:      status,
	}))

	tlEntries, err := repo.TimelineEntries(ctx, seymour.TimelineEntriesArgs{UserID: userID, FeedID: feed.ID})
	require.NoError(t, err)
	require.Len(t, tlEntries, 1)

	return tlEntries[0].ID
}

func TestTimelineEntries_FilterByStatus(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	seedTimelineEntry(t, repo, "user-1", "approved-entry", now, seymour.TimelineEntryStatusApproved)
	seedTimelineEntry(t, repo, "user-1", "pending-entry", now, seymour.TimelineEntryStatusRequiresJudgement)

	// No status filter: everything comes back.
	all, err := repo.TimelineEntries(ctx, seymour.TimelineEntriesArgs{UserID: "user-1"})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	count, err := repo.CountTimelineEntries(ctx, seymour.TimelineEntriesArgs{UserID: "user-1"})
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Narrowed to approved only.
	approved, err := repo.TimelineEntries(ctx, seymour.TimelineEntriesArgs{UserID: "user-1", Status: seymour.TimelineEntryStatusApproved})
	require.NoError(t, err)
	require.Len(t, approved, 1)
	assert.Equal(t, seymour.TimelineEntryStatusApproved, approved[0].Status)
}

func TestTimelineEntries_FilterByDateRange(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	inRange := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	seedTimelineEntry(t, repo, "user-1", "older-entry", older, seymour.TimelineEntryStatusApproved)
	seedTimelineEntry(t, repo, "user-1", "in-range-entry", inRange, seymour.TimelineEntryStatusApproved)
	seedTimelineEntry(t, repo, "user-1", "newer-entry", newer, seymour.TimelineEntryStatusApproved)

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	entries, err := repo.TimelineEntries(ctx, seymour.TimelineEntriesArgs{
		UserID:   "user-1",
		FromDate: &from,
		ToDate:   &to,
	})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	feedEntries, err := repo.Entries(ctx, []string{entries[0].FeedEntryID})
	require.NoError(t, err)
	require.Len(t, feedEntries, 1)
	assert.Equal(t, "in-range-entry", feedEntries[0].GUID)

	count, err := repo.CountTimelineEntries(ctx, seymour.TimelineEntriesArgs{
		UserID:   "user-1",
		FromDate: &from,
		ToDate:   &to,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
