package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	readability "github.com/go-shiori/go-readability"
	"github.com/gorilla/mux"
	"github.com/sym01/htmlsanitizer"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
	"github.com/jdholdren/seymour/internal/worker"
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

	// Start the workflow to create it and verify it
	subscriptionID, err := worker.SubscribeToFeed(ctx, s.tempCli, body.FeedURL, userID)
	var seyErr *seymour.Error
	if errors.As(err, &seyErr) {
		return seyErr
	}
	if err != nil {
		return err
	}

	subscription, err := s.timeline.Subscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	feed, err := s.feeds.Feed(ctx, subscription.FeedID)
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

	subs, err := s.timeline.AllSubscriptions(ctx, userID)
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

	sub, err := s.timeline.Subscription(ctx, id)
	if err != nil {
		return err
	}
	if sub.UserID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	if err := s.timeline.DeleteSubscription(ctx, id); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func apiFilter(f seymour.Filter) apiv1.Filter {
	switch cfg := f.(type) {
	case seymour.AllowListConfig:
		return apiv1.Filter{
			ID:              cfg.ID,
			Type:            apiv1.FilterType(cfg.Type()),
			AllowListConfig: &apiv1.AllowListConfig{Keywords: cfg.Keywords},
		}
	case seymour.DisallowListConfig:
		return apiv1.Filter{
			ID:                 cfg.ID,
			Type:               apiv1.FilterType(cfg.Type()),
			DisallowListConfig: &apiv1.DisallowListConfig{Keywords: cfg.Keywords},
		}
	default:
		return apiv1.Filter{Type: apiv1.FilterType(f.Type())}
	}
}

// filterUserID pulls the owning user's ID back out of a [seymour.Filter],
// type-switching the same way [Repo.CreateFilter] does when persisting one.
func filterUserID(f seymour.Filter) string {
	switch cfg := f.(type) {
	case seymour.AllowListConfig:
		return cfg.UserID
	case seymour.DisallowListConfig:
		return cfg.UserID
	default:
		return ""
	}
}

func (s Server) getUserFilters(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx    = r.Context()
		userID = mux.Vars(r)["userID"]
	)
	if userID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	filters, err := s.timeline.UserFilters(ctx, userID)
	if err != nil {
		return err
	}

	resp := apiv1.UserFiltersResp{
		Filters: make([]apiv1.Filter, 0, len(filters)),
	}
	for _, f := range filters {
		resp.Filters = append(resp.Filters, apiFilter(f))
	}

	return writeJSON(w, http.StatusOK, resp)
}

func (s Server) deleteFilter(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx      = r.Context()
		filterID = mux.Vars(r)["filterID"]
	)

	f, err := s.timeline.Filter(ctx, filterID)
	if err != nil {
		return err
	}
	if filterUserID(f) != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	if err := s.timeline.DeleteFilter(ctx, filterID); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// validTimelineEntryStatuses are the values getTimeline accepts for its
// ?status= filter.
var validTimelineEntryStatuses = map[seymour.TimelineEntryStatus]bool{
	seymour.TimelineEntryStatusRequiresJudgement: true,
	seymour.TimelineEntryStatusApproved:          true,
	seymour.TimelineEntryStatusRejected:          true,
}

func (s Server) getTimeline(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx    = r.Context()
		userID = mux.Vars(r)["userID"]
		query  = r.URL.Query()
		feedID = query.Get("feed_id")
		status = seymour.TimelineEntryStatus(query.Get("status"))
	)
	if userID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}
	if status != "" && !validTimelineEntryStatuses[status] {
		return seymour.E(fmt.Sprintf("invalid status: %s", status), http.StatusBadRequest)
	}

	var fromDate apiv1.Date
	if v := query.Get("from"); v != "" {
		d, err := apiv1.ParseDate(v)
		if err != nil {
			return seymour.E(fmt.Sprintf("invalid from: %s", err), http.StatusBadRequest)
		}
		fromDate = d
	}
	var toDate apiv1.Date
	if v := query.Get("to"); v != "" {
		d, err := apiv1.ParseDate(v)
		if err != nil {
			return seymour.E(fmt.Sprintf("invalid to: %s", err), http.StatusBadRequest)
		}
		toDate = d
	}

	// Parse pagination parameters
	limit, offset := parsePaginationParams(r, 20, 100) // default=20, max=100

	args := seymour.TimelineEntriesArgs{
		UserID:   userID,
		Status:   status,
		FeedID:   feedID,
		FromDate: fromDate,
		ToDate:   toDate,
		Limit:    uint64(limit),
		Offset:   uint64(offset),
	}

	// Get count and entries
	total, err := s.timeline.CountTimelineEntries(ctx, args)
	if err != nil {
		return err
	}

	tlEnts, err := s.timeline.TimelineEntries(ctx, args)
	if err != nil {
		return err
	}

	feedEntIDs := make([]string, 0, len(tlEnts))
	for _, ent := range tlEnts {
		feedEntIDs = append(feedEntIDs, ent.FeedEntryID)
	}

	feedEnts, err := s.feeds.Entries(ctx, feedEntIDs)
	if err != nil {
		return err
	}

	feedIDs := make([]string, 0, len(feedEnts))
	for _, ent := range feedEnts {
		feedIDs = append(feedIDs, ent.FeedID)
	}

	feeds, err := s.feeds.Feeds(ctx, feedIDs)
	if err != nil {
		return err
	}

	// Turn into a maps for fast lookup
	var (
		feedByID        = make(map[string]seymour.Feed)
		feedEntriesByID = make(map[string]seymour.FeedEntry)
	)
	for _, feed := range feeds {
		feedByID[feed.ID] = feed
	}
	for _, feedEntry := range feedEnts {
		feedEntriesByID[feedEntry.ID] = feedEntry
	}

	// Build timeline entries
	items := make([]apiv1.TimelineEntry, 0, len(tlEnts))
	for _, tlEntry := range tlEnts {
		var (
			feedEntry = feedEntriesByID[tlEntry.FeedEntryID]
			feed      = feedByID[feedEntry.FeedID]
			feedTitle string
		)
		if feed.Title != nil {
			feedTitle = *feed.Title
		}

		items = append(items, apiv1.TimelineEntry{
			EntryID:     feedEntry.ID,
			FeedName:    feedTitle,
			Title:       feedEntry.Title,
			Description: feedEntry.Description,
			URL:         feedEntry.Link,
			PublishDate: feedEntry.PublishTime,
		})
	}

	// Build pagination metadata
	resp := apiv1.TimelineResp{
		Items:      items,
		Pagination: calculatePaginationMeta(limit, offset, total),
	}

	return writeJSON(w, http.StatusOK, resp)
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
