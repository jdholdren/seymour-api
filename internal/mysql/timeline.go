package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jdholdren/seymour/internal/seymour"
)

const (
	subscriptionNamespace = "sub"
)

func (r Repo) CreateSubscription(ctx context.Context, userID, feedID string) (string, error) {
	const q = `INSERT INTO subscriptions (id, user_id, feed_id) VALUES (?, ?, ?);`

	id := fmt.Sprintf("%s-%s", uuid.New().String(), subscriptionNamespace)
	if _, err := r.db.ExecContext(ctx, q, id, userID, feedID); err != nil {
		return "", fmt.Errorf("error creating subscription: %w", err)
	}

	return id, nil
}

func (r Repo) AllSubscriptions(ctx context.Context, userID string) ([]seymour.Subscription, error) {
	const q = `SELECT * FROM subscriptions WHERE user_id = ?;`

	var subs []seymour.Subscription
	if err := r.db.SelectContext(ctx, &subs, q, userID); err != nil {
		return nil, fmt.Errorf("error selecting subscriptions: %s", err)
	}

	return subs, nil
}

func (r Repo) DeleteSubscription(ctx context.Context, id string) error {
	const q = `DELETE FROM subscriptions WHERE id = ?;`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("error deleting subscription: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}
	if n == 0 {
		return seymour.ErrNotFound
	}

	return nil
}

func (r Repo) Subscription(ctx context.Context, id string) (seymour.Subscription, error) {
	const q = `SELECT * FROM subscriptions WHERE id = ?;`

	var sub seymour.Subscription
	err := r.db.GetContext(ctx, &sub, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return seymour.Subscription{}, seymour.ErrNotFound
	}
	if err != nil {
		return seymour.Subscription{}, fmt.Errorf("error selecting subscription: %s", err)
	}

	return sub, nil
}
