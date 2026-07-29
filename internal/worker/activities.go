package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	seyerrs "github.com/jdholdren/seymour/internal/errors"
	"github.com/jdholdren/seymour/internal/seymour"
	"github.com/jdholdren/seymour/internal/sync"
)

type activities struct {
	repo         seymour.Repository
	claudeClient *anthropic.Client
}

// Instance to make the workflow a bit more readable
var acts = activities{}

// Count all feeds we know about in the system.
//
// Used for batching up the work as the number of feeds grows.
func (a activities) CountAllFeeds(ctx context.Context) (int, error) {
	n, err := a.repo.CountAllFeeds(ctx)
	if err != nil {
		return 0, err
	}

	return n, nil
}

// FeedIDPage fetches a page of feed ids from the DB.
//
// Useful for batching work across the global set of feeds.
func (a activities) FeedIDPage(ctx context.Context, offset, pageSize int) ([]string, error) {
	ids, err := a.repo.FeedIDs(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// Goes to the url and grabs the RSS feed items.
func (a activities) SyncFeed(ctx context.Context, feedID string, ignoreRecency bool) error {
	feed, err := a.repo.Feed(ctx, feedID)
	if err != nil {
		return err
	}

	// If recently synced, exit early, don't repeat work:
	if !ignoreRecency && feed.LastSyncedAt != nil && time.Since(feed.LastSyncedAt.Time) < time.Hour {
		return nil
	}

	feed, entries, err := sync.Feed(ctx, feed.ID, feed.URL)
	if err != nil {
		return temporal.NewApplicationError("error syncing feed", "seyerr", seyerrs.E(err, http.StatusBadRequest))
	}

	if err := a.repo.UpdateFeed(ctx, feed.ID, seymour.UpdateFeedArgs{
		Title:       *feed.Title,
		Description: *feed.Description,
		LastSynced:  seymour.DBTime{Time: time.Now()},
	}); err != nil {
		return err
	}
	if err := a.repo.InsertEntries(ctx, entries); err != nil {
		return err
	}

	return err
}

func (a activities) CreateFeed(ctx context.Context, feedURL string) (string, error) {
	feed, err := a.repo.InsertFeed(ctx, feedURL)
	if errors.Is(err, seymour.ErrConflict) {
		// Fetch the feed from the database
		feed, err = a.repo.FeedByURL(ctx, feedURL)
		if err != nil {
			return "", fmt.Errorf("error fetching conflicting feed: %s", err)
		}

		return feed.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("error inserting feed: %w", err)
	}

	return feed.ID, nil
}

func (a activities) RemoveFeed(ctx context.Context, feedID string) error {
	if err := a.repo.DeleteFeed(ctx, feedID); err != nil {
		return fmt.Errorf("error deleting feed: %w", err)
	}

	return nil
}

// Inserts timeline entries that should be present in the timeline based on subscriptions but are missing.
//
// Returns number of missing entries inserted.
func (a activities) InsertMissingTimelineEntries(ctx context.Context) (int, error) {
	l := activity.GetLogger(ctx)

	missing, err := a.repo.MissingEntries(ctx)
	if err != nil {
		return 0, fmt.Errorf("error finding missing timeline entries: %s", err)
	}

	l.Info("searched for missing timeline entries", "length", len(missing))

	// Keep track of affected entries
	for _, m := range missing {
		if err := a.repo.InsertEntry(ctx, seymour.TimelineEntry{
			FeedEntryID: m.FeedEntryID,
			Status:      seymour.TimelineEntryStatusRequiresJudgement,
			FeedID:      m.FeedID,
		}); err != nil {
			return 0, fmt.Errorf("error inserting timeline entry: %w", err)
		}
	}

	return len(missing), nil
}

// CountEntriesNeedingJudgement checks the current count of how many entries need judgement.
func (a activities) CountEntriesNeedingJudgement(ctx context.Context) (uint, error) {
	entries, err := a.repo.EntriesNeedingJudgement(ctx, 1000)
	if err != nil {
		return 0, fmt.Errorf("error finding entries needing judgement: %s", err)
	}

	return uint(len(entries)), nil
}

// Type that holds a timeline entry ID and whether it has been approved.
type judgements map[string]bool

func (a activities) MarkEntriesAsJudged(ctx context.Context, js judgements) error {
	for timelineEntryID, approved := range js {
		status := seymour.TimelineEntryStatusRejected
		if approved {
			status = seymour.TimelineEntryStatusApproved
		}

		if err := a.repo.UpdateTimelineEntry(ctx, timelineEntryID, status); err != nil {
			return fmt.Errorf("error updating timeline entry status: %w", err)
		}
	}

	return nil
}

// AllUserIDs is no longer needed in single-tenant mode, return empty slice
func (a activities) AllUserIDs(ctx context.Context) ([]string, error) {
	return []string{}, nil
}
