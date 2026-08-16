package mysql

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jdholdren/seymour/internal/seymour"
)

const (
	filterNamespace        = "fltr"
	filterKeywordNamespace = "fltr-kw"
)

// userFilterRow mirrors a row of the root user_filters table; the type then
// decides which other tables get batched in to assemble the final
// [seymour.Filter].
type userFilterRow struct {
	ID     string             `db:"id"`
	UserID string             `db:"user_id"`
	Type   seymour.FilterType `db:"type"`
}

// keywordRow mirrors a row of filter_keywords, the table shared by
// allow_list and disallow_list filters.
type keywordRow struct {
	FilterID string `db:"filter_id"`
	Keyword  string `db:"keyword"`
}

// webhookConfigRow mirrors a row of filter_webhooks, backing WebhookConfig.
type webhookConfigRow struct {
	FilterID string `db:"filter_id"`
	Host     string `db:"host"`
}

func (r Repo) CreateFilter(ctx context.Context, userID string, filter seymour.Filter) (string, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	id := fmt.Sprintf("%s-%s", uuid.New().String(), filterNamespace)
	const insertFilterQ = `INSERT INTO user_filters (id, user_id, type) VALUES (?, ?, ?);`
	if _, err := tx.ExecContext(ctx, insertFilterQ, id, userID, filter.Type()); err != nil {
		return "", fmt.Errorf("error inserting filter: %w", err)
	}

	switch f := filter.(type) {
	case seymour.AllowListConfig:
		if err := insertFilterKeywords(ctx, tx, id, f.Keywords); err != nil {
			return "", err
		}
	case seymour.DisallowListConfig:
		if err := insertFilterKeywords(ctx, tx, id, f.Keywords); err != nil {
			return "", err
		}
	case seymour.WebhookConfig:
		const insertWebhookQ = `INSERT INTO filter_webhooks (filter_id, host) VALUES (?, ?);`
		if _, err := tx.ExecContext(ctx, insertWebhookQ, id, f.Host); err != nil {
			return "", fmt.Errorf("error inserting webhook filter: %w", err)
		}
	default:
		return "", seymour.E(fmt.Sprintf("unsupported filter type: %s", filter.Type()), 400)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("error committing transaction: %w", err)
	}

	return id, nil
}

func insertFilterKeywords(ctx context.Context, tx *sqlx.Tx, filterID string, keywords []string) error {
	const q = `INSERT INTO filter_keywords (id, filter_id, keyword) VALUES (?, ?, ?);`
	for _, kw := range keywords {
		id := fmt.Sprintf("%s-%s", uuid.New().String(), filterKeywordNamespace)
		if _, err := tx.ExecContext(ctx, q, id, filterID, kw); err != nil {
			return fmt.Errorf("error inserting filter keyword: %w", err)
		}
	}

	return nil
}

func (r Repo) UserFilters(ctx context.Context, userID string) ([]seymour.Filter, error) {
	const q = `SELECT id, user_id, type FROM user_filters WHERE user_id = ?;`

	var rows []userFilterRow
	if err := r.db.SelectContext(ctx, &rows, q, userID); err != nil {
		return nil, fmt.Errorf("error selecting user filters: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var keywordFilterIDs, webhookFilterIDs []string
	for _, row := range rows {
		switch row.Type {
		case seymour.FilterTypeAllowList, seymour.FilterTypeDisallowList:
			keywordFilterIDs = append(keywordFilterIDs, row.ID)
		case seymour.FilterTypeWebhook:
			webhookFilterIDs = append(webhookFilterIDs, row.ID)
		}
	}

	keywordRows, err := r.keywordRowsByFilterID(ctx, keywordFilterIDs)
	if err != nil {
		return nil, err
	}
	keywordsByFilter := make(map[string][]string, len(keywordFilterIDs))
	for _, kwRow := range keywordRows {
		keywordsByFilter[kwRow.FilterID] = append(keywordsByFilter[kwRow.FilterID], kwRow.Keyword)
	}

	webhookRows, err := r.webhookConfigRowsByFilterID(ctx, webhookFilterIDs)
	if err != nil {
		return nil, err
	}
	hostByFilter := make(map[string]string, len(webhookFilterIDs))
	for _, whRow := range webhookRows {
		hostByFilter[whRow.FilterID] = whRow.Host
	}

	filters := make([]seymour.Filter, 0, len(rows))
	for _, row := range rows {
		switch row.Type {
		case seymour.FilterTypeAllowList:
			filters = append(filters, seymour.AllowListConfig{
				ID:       row.ID,
				UserID:   row.UserID,
				Keywords: keywordsByFilter[row.ID],
			})
		case seymour.FilterTypeDisallowList:
			filters = append(filters, seymour.DisallowListConfig{
				ID:       row.ID,
				UserID:   row.UserID,
				Keywords: keywordsByFilter[row.ID],
			})
		case seymour.FilterTypeWebhook:
			filters = append(filters, seymour.WebhookConfig{
				ID:     row.ID,
				UserID: row.UserID,
				Host:   hostByFilter[row.ID],
			})
		}
	}

	return filters, nil
}

func (r Repo) keywordRowsByFilterID(ctx context.Context, filterIDs []string) ([]keywordRow, error) {
	if len(filterIDs) == 0 {
		return nil, nil
	}

	query, args, err := sq.Select("filter_id", "keyword").
		From("filter_keywords").
		Where(sq.Eq{"filter_id": filterIDs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("error constructing sql: %s", err)
	}

	var rows []keywordRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("error selecting filter keywords: %w", err)
	}

	return rows, nil
}

func (r Repo) webhookConfigRowsByFilterID(ctx context.Context, filterIDs []string) ([]webhookConfigRow, error) {
	if len(filterIDs) == 0 {
		return nil, nil
	}

	query, args, err := sq.Select("filter_id", "host").
		From("filter_webhooks").
		Where(sq.Eq{"filter_id": filterIDs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("error constructing sql: %s", err)
	}

	var rows []webhookConfigRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("error selecting filter webhooks: %w", err)
	}

	return rows, nil
}
