package api

import (
	"net/http"
)

type (
	Viewer struct {
		Subscriptions map[string]ViewerSubscription `json:"subscriptions"`
	}

	ViewerSubscription struct {
		Name        string `json:"name"`
		FeedID      string `json:"feed_id"`
		Description string `json:"description"`
	}
)

func (s Server) handleViewer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	subs, err := s.repo.AllSubscriptions(ctx)
	if err != nil {
		return err
	}

	var feedIDs []string
	for _, sub := range subs {
		feedIDs = append(feedIDs, sub.FeedID)
	}

	feeds, err := s.repo.Feeds(ctx, feedIDs)
	if err != nil {
		return err
	}

	viewerSubs := make(map[string]ViewerSubscription)
	for _, feed := range feeds {
		var title, desc string
		if feed.Title != nil {
			title = *feed.Title
		}
		if feed.Description != nil {
			desc = *feed.Description
		}

		viewerSubs[feed.ID] = ViewerSubscription{
			Name:        title,
			FeedID:      feed.ID,
			Description: desc,
		}
	}

	return writeJSON(w, http.StatusOK, Viewer{
		Subscriptions: viewerSubs,
	})
}
