package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	readability "github.com/go-shiori/go-readability"
	"github.com/gorilla/mux"
	"github.com/sym01/htmlsanitizer"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
	"github.com/jdholdren/seymour/internal/sync"
)

func validatePostSubscriptionReq(req apiv1.PostSubscriptionReq) error {
	if req.FeedURL == "" {
		return seymour.E("feed_url is required", http.StatusBadRequest)
	}

	return nil
}

// apiSubscription converges a subscription with its feed into the response shape.
func apiSubscription(sub seymour.Subscription, feed seymour.Feed) apiv1.SubscriptionResp {
	var (
		feedName        string
		feedDescription string
		lastSynced      *time.Time
	)
	if feed.Title != nil {
		feedName = *feed.Title
	}
	if feed.Description != nil {
		feedDescription = *feed.Description
	}
	if feed.LastSyncedAt != nil {
		lastSynced = feed.LastSyncedAt
	}

	return apiv1.SubscriptionResp{
		ID:              sub.ID,
		FeedID:          sub.FeedID,
		CreatedAt:       sub.CreatedAt,
		FeedName:        feedName,
		FeedDescription: feedDescription,
		LastSynced:      lastSynced,
	}
}

func (s Server) postSusbcriptions(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx    = r.Context()
		userID = mux.Vars(r)["userID"]
		body   apiv1.PostSubscriptionReq
	)
	if userID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return seymour.E(err, http.StatusBadRequest)
	}
	if err := validatePostSubscriptionReq(body); err != nil {
		return err
	}

	// Reuse the shared feed cache, ingest it before adding the subscription,
	// then persist the user's subscription synchronously.
	feed, err := s.feeds.InsertFeed(ctx, body.FeedURL)
	if errors.Is(err, seymour.ErrConflict) {
		feed, err = s.feeds.FeedByURL(ctx, body.FeedURL)
	}
	if err != nil {
		return err
	}

	parsedFeed, entries, err := sync.Feed(ctx, feed.ID, feed.URL)
	if err != nil {
		return err
	}
	if err := s.feeds.InsertEntries(ctx, entries); err != nil {
		return err
	}
	if parsedFeed.Title != nil && parsedFeed.Description != nil {
		if err := s.feeds.UpdateFeed(ctx, feed.ID, seymour.UpdateFeedArgs{
			Title: *parsedFeed.Title, Description: *parsedFeed.Description, LastSynced: time.Now(),
		}); err != nil {
			return err
		}
	}
	subscriptionID, err := s.subscriptions.CreateSubscription(ctx, userID, feed.ID)
	if err != nil {
		return err
	}

	subscription, err := s.subscriptions.Subscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	feed, err = s.feeds.Feed(ctx, subscription.FeedID)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusCreated, apiSubscription(subscription, feed))
}

func (s Server) getSusbcriptions(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx    = r.Context()
		userID = mux.Vars(r)["userID"]
	)
	if userID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	subs, err := s.subscriptions.AllSubscriptions(ctx, userID)
	if err != nil {
		return err
	}

	resp := apiv1.SubscriptionListResp{
		Subscriptions: []apiv1.SubscriptionResp{},
	}
	for _, sub := range subs {
		// Totally inefficient, yet sufficient:
		feed, err := s.feeds.Feed(ctx, sub.FeedID)
		if err != nil {
			return err
		}

		resp.Subscriptions = append(resp.Subscriptions, apiSubscription(sub, feed))
	}
	return writeJSON(w, http.StatusCreated, resp)
}

func (s Server) deleteSubscription(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx = r.Context()
		id  = mux.Vars(r)["subscriptionID"]
	)

	sub, err := s.subscriptions.Subscription(ctx, id)
	if err != nil {
		return err
	}
	if sub.UserID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	if err := s.subscriptions.DeleteSubscription(ctx, id); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s Server) getFeedEntry(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx         = r.Context()
		feedEntryID = mux.Vars(r)["feedEntryID"]
	)

	entry, err := s.feeds.Entry(ctx, feedEntryID)
	if err != nil {
		return err
	}

	// Cache results for less processing and prevent refetches
	if resp, ok := s.entryRespCache.Get(feedEntryID); ok {
		return writeJSON(w, http.StatusOK, resp)
	}

	// TODO: Ensure this at sync time in the workflow
	u, err := url.Parse(entry.GUID)
	if err != nil {
		return fmt.Errorf("error with the feed entry's url: %s", err)
	}

	// Fetch the actual site
	resp, err := s.fetchClient.Get(entry.Link)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Strip it for readability and sanitize
	parser := readability.NewParser()
	article, err := parser.Parse(resp.Body, u)
	if err != nil {
		return err
	}

	santizer := htmlsanitizer.NewHTMLSanitizer()
	contents, err := santizer.SanitizeString(article.Content)
	if err != nil {
		return err
	}

	ret := apiv1.FeedEntryResp{
		ID:            entry.ID,
		FeedID:        entry.FeedID,
		URL:           entry.Link,
		Title:         entry.Title,
		Description:   entry.Description,
		CreatedAt:     entry.CreatedAt,
		ReaderContent: contents,
	}
	// Add to the cache for next time
	s.entryRespCache.Add(entry.ID, ret)

	return writeJSON(w, http.StatusOK, ret)
}

func (s Server) getFeedEntries(w http.ResponseWriter, r *http.Request) error {
	// Feed entries are shared catalog data; authentication is enforced by the
	// protected router, without restricting access to the feed's subscriber.
	page, size := 1, 20
	if value := r.URL.Query().Get("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return seymour.E("page must be a positive integer", http.StatusBadRequest)
		}
		page = parsed
	}
	if value := r.URL.Query().Get("size"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			return seymour.E("size must be an integer between 1 and 100", http.StatusBadRequest)
		}
		size = parsed
	}
	if page-1 > int(^uint(0)>>1)/size {
		return seymour.E("page is too large", http.StatusBadRequest)
	}
	offset := (page - 1) * size

	feedID := mux.Vars(r)["feedID"]
	entries, err := s.feeds.FeedEntries(r.Context(), feedID, offset, size)
	if err != nil {
		return err
	}
	total, err := s.feeds.CountFeedEntries(r.Context(), feedID)
	if err != nil {
		return err
	}

	resp := apiv1.FeedEntriesResp{
		Page:    page,
		Size:    size,
		Total:   total,
		Entries: make([]apiv1.FeedEntrySummaryResp, 0, len(entries)),
	}
	for _, entry := range entries {
		resp.Entries = append(resp.Entries, apiv1.FeedEntrySummaryResp{
			ID:          entry.ID,
			FeedID:      entry.FeedID,
			URL:         entry.Link,
			Title:       entry.Title,
			Description: entry.Description,
			CreatedAt:   entry.CreatedAt,
			PublishTime: entry.PublishTime,
		})
	}

	return writeJSON(w, http.StatusOK, resp)
}
