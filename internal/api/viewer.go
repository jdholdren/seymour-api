package api

import (
	"net/http"

	viewerv1 "github.com/jdholdren/seymour/apis/viewer/v1"
)

func (s Server) handleViewer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	subs, err := s.timeline.AllSubscriptions(ctx)
	if err != nil {
		return err
	}

	var feedIDs []string
	for _, sub := range subs {
		feedIDs = append(feedIDs, sub.FeedID)
	}

	feeds, err := s.feeds.Feeds(ctx, feedIDs)
	if err != nil {
		return err
	}

	viewerSubs := make(map[string]viewerv1.ViewerSubscription)
	for _, feed := range feeds {
		var title, desc string
		if feed.Title != nil {
			title = *feed.Title
		}
		if feed.Description != nil {
			desc = *feed.Description
		}

		viewerSubs[feed.ID] = viewerv1.ViewerSubscription{
			Name:        title,
			FeedID:      feed.ID,
			Description: desc,
		}
	}

	return writeJSON(w, http.StatusOK, viewerv1.Viewer{
		Subscriptions: viewerSubs,
	})
}
