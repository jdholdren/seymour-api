package sqlite

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/jdholdren/seymour/internal/seymour"
)

const (
	subscriptionNamespace  = "sub"
	timelineEntryNamespace = "tl-entry"
)

func (r Repo) CreateSubscription(ctx context.Context, feedID string) error {
	const q = `INSERT OR IGNORE INTO subscriptions (id, feed_id) VALUES (?, ?);`

	id := fmt.Sprintf("%s-%s", uuid.New().String(), subscriptionNamespace)
	if _, err := r.db.ExecContext(ctx, q, id, feedID); err != nil {
		return fmt.Errorf("error creating subscription: %w", err)
	}

	return nil
}

func (r Repo) AllSubscriptions(ctx context.Context) ([]seymour.Subscription, error) {
	const q = `SELECT * FROM subscriptions;`

	var subs []seymour.Subscription
	if err := r.db.SelectContext(ctx, &subs, q); err != nil {
		return nil, fmt.Errorf("error selecting subscriptions: %s", err)
	}

	return subs, nil
}

func (r Repo) Subscription(ctx context.Context, feedID string) (*seymour.Subscription, error) {
	const q = `SELECT * FROM subscriptions WHERE feed_id = ?;`

	var sub seymour.Subscription
	if err := r.db.GetContext(ctx, &sub, q, feedID); err != nil {
		return nil, fmt.Errorf("error selecting subscription: %s", err)
	}

	return &sub, nil
}

func (r Repo) MissingEntries(ctx context.Context) ([]seymour.MissingEntry, error) {
	const q = `
	SELECT
		fe.feed_id AS feed_id,
		fe.id AS feed_entry_id
	FROM
		feed_entries fe
		INNER JOIN feeds ON feeds.id = fe.feed_id
		INNER JOIN subscriptions subs ON subs.feed_id = feeds.id
		LEFT JOIN timeline_entries ts ON ts.feed_entry_id = fe.id
		WHERE ts.feed_entry_id IS NULL ;
	`

	var missingEntries []seymour.MissingEntry
	if err := r.db.SelectContext(ctx, &missingEntries, q); err != nil {
		return nil, fmt.Errorf("error selecting missing entries: %s", err)
	}

	return missingEntries, nil
}

func (r Repo) InsertEntry(ctx context.Context, entry seymour.TimelineEntry) error {
	const q = `INSERT OR IGNORE INTO timeline_entries (
		id,
		feed_entry_id,
		status,
		feed_id
	) VALUES (
		?,
		?,
		?,
		?
	);
	`

	entry.ID = fmt.Sprintf("%s-%s", uuid.New().String(), timelineEntryNamespace)
	if _, err := r.db.ExecContext(ctx, q, entry.ID, entry.FeedEntryID, entry.Status, entry.FeedID); err != nil {
		return fmt.Errorf("error inserting entry: %s", err)
	}

	return nil
}

func (r Repo) EntriesNeedingJudgement(ctx context.Context, limit uint) ([]seymour.TimelineEntry, error) {
	const q = `
	SELECT
		id,
		feed_entry_id,
		created_at,
		status,
		feed_id
	FROM
		timeline_entries
	WHERE
		status = ?
	LIMIT ?;
	`

	var entries []seymour.TimelineEntry
	if err := r.db.SelectContext(ctx, &entries, q, seymour.TimelineEntryStatusRequiresJudgement, limit); err != nil {
		return nil, fmt.Errorf("error selecting entries needing judgement: %s", err)
	}

	return entries, nil
}

func (r Repo) UpdateTimelineEntry(ctx context.Context, id string, status seymour.TimelineEntryStatus) error {
	const q = `UPDATE timeline_entries SET status = ? WHERE id = ?;`
	if _, err := r.db.ExecContext(ctx, q, status, id); err != nil {
		return fmt.Errorf("error updating entry: %s", err)
	}

	return nil
}

func (r Repo) TimelineEntries(ctx context.Context, args seymour.TimelineEntriesArgs) ([]seymour.TimelineEntry, error) {
	q := sq.Select("id", "feed_entry_id", "created_at", "status", "feed_id").From("timeline_entries").OrderBy("created_at DESC")
	where := sq.Eq{}
	if args.Status != "" {
		where["status"] = args.Status
	}
	if args.Limit > 0 {
		q = q.Limit(args.Limit)
	}
	if args.Offset > 0 {
		q = q.Offset(args.Offset)
	}
	if args.FeedID != "" {
		q = q.Where("feed_id = ?", args.FeedID)
	}

	if len(where) > 0 {
		q = q.Where(where)
	}

	query, queryArgs, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error generating SQL query: %s", err)
	}

	var entries []seymour.TimelineEntry
	if err := r.db.SelectContext(ctx, &entries, query, queryArgs...); err != nil {
		return nil, fmt.Errorf("error selecting timeline entries: %s", err)
	}

	return entries, nil
}

func (r Repo) CountTimelineEntries(ctx context.Context, args seymour.TimelineEntriesArgs) (int, error) {
	q := sq.Select("COUNT(*)").From("timeline_entries")
	where := sq.Eq{}
	if args.Status != "" {
		where["status"] = args.Status
	}
	if args.FeedID != "" {
		q = q.Where("feed_id = ?", args.FeedID)
	}

	if len(where) > 0 {
		q = q.Where(where)
	}

	query, queryArgs, err := q.ToSql()
	if err != nil {
		return 0, fmt.Errorf("error generating SQL query: %s", err)
	}

	var count int
	if err := r.db.GetContext(ctx, &count, query, queryArgs...); err != nil {
		return 0, fmt.Errorf("error counting timeline entries: %s", err)
	}

	return count, nil
}
