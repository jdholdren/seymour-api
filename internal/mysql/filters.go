package mysql

import (
	"context"
	"database/sql"
	"errors"
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

	var keywords []string
	switch f := filter.(type) {
	case seymour.AllowListConfig:
		keywords = f.Keywords
	case seymour.DisallowListConfig:
		keywords = f.Keywords
	default:
		return "", seymour.E(fmt.Sprintf("unsupported filter type: %s", filter.Type()), 400)
	}
	if err := insertFilterKeywords(ctx, tx, id, keywords); err != nil {
		return "", err
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

	return r.filtersFromRows(ctx, rows)
}

func (r Repo) Filter(ctx context.Context, id string) (seymour.Filter, error) {
	const q = `SELECT id, user_id, type FROM user_filters WHERE id = ?;`

	var row userFilterRow
	err := r.db.GetContext(ctx, &row, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, seymour.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error selecting filter: %w", err)
	}

	filters, err := r.filtersFromRows(ctx, []userFilterRow{row})
	if err != nil {
		return nil, err
	}

	return filters[0], nil
}

func (r Repo) DeleteFilter(ctx context.Context, id string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const deleteKeywordsQ = `DELETE FROM filter_keywords WHERE filter_id = ?;`
	if _, err := tx.ExecContext(ctx, deleteKeywordsQ, id); err != nil {
		return fmt.Errorf("error deleting filter keywords: %w", err)
	}

	const deleteFilterQ = `DELETE FROM user_filters WHERE id = ?;`
	res, err := tx.ExecContext(ctx, deleteFilterQ, id)
	if err != nil {
		return fmt.Errorf("error deleting filter: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}
	if n == 0 {
		return seymour.ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// filtersFromRows batches the keywords for the given user_filters rows and
// assembles them into the polymorphic [seymour.Filter] configs.
func (r Repo) filtersFromRows(ctx context.Context, rows []userFilterRow) ([]seymour.Filter, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	filterIDs := make([]string, len(rows))
	for i, row := range rows {
		filterIDs[i] = row.ID
	}

	keywordRows, err := r.keywordRowsByFilterID(ctx, filterIDs)
	if err != nil {
		return nil, err
	}
	keywordsByFilter := make(map[string][]string, len(filterIDs))
	for _, kwRow := range keywordRows {
		keywordsByFilter[kwRow.FilterID] = append(keywordsByFilter[kwRow.FilterID], kwRow.Keyword)
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
