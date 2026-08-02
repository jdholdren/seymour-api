package api

import (
	"net/http"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
)

func (s Server) handleViewer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	userID, ok := s.sessionUserID(r)
	if !ok {
		return writeJSON(w, http.StatusOK, apiv1.Viewer{
			Subscriptions: map[string]apiv1.ViewerSubscription{},
		})
	}

	user, err := s.users.User(ctx, userID)
	if err != nil {
		return err
	}

	subs, err := s.timeline.AllSubscriptions(ctx, userID)
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

	viewerSubs := make(map[string]apiv1.ViewerSubscription)
	for _, feed := range feeds {
		var title, desc string
		if feed.Title != nil {
			title = *feed.Title
		}
		if feed.Description != nil {
			desc = *feed.Description
		}

		viewerSubs[feed.ID] = apiv1.ViewerSubscription{
			Name:        title,
			FeedID:      feed.ID,
			Description: desc,
		}
	}

	var preferredName string
	if user.PreferredName != nil {
		preferredName = *user.PreferredName
	}

	return writeJSON(w, http.StatusOK, apiv1.Viewer{
		User: &apiv1.ViewerUser{
			ID:            user.ID,
			PreferredName: preferredName,
		},
		Subscriptions: viewerSubs,
	})
}
