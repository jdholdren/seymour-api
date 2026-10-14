package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/jdholdren/seymour/internal/seymour"
)

const filterNamespace = "-fl"

type filterRow struct {
	ID        string             `db:"id"`
	UserID    string             `db:"user_id"`
	Type      seymour.FilterType `db:"type"`
	Enabled   bool               `db:"enabled"`
	Config    []byte             `db:"config"`
	CreatedAt time.Time          `db:"created_at"`
	UpdatedAt time.Time          `db:"updated_at"`
}

func (row filterRow) model() (seymour.Filter, error) {
	filter := seymour.Filter{
		ID: row.ID, UserID: row.UserID, Type: row.Type, Enabled: row.Enabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	switch row.Type {
	case seymour.FilterTypeAI:
		var config seymour.AIConfig
		if err := json.Unmarshal(row.Config, &config); err != nil {
			return seymour.Filter{}, fmt.Errorf("decode ai filter config: %w", err)
		}
		filter.AIConfig = &config
	case seymour.FilterTypeList:
		var config seymour.ListConfig
		if err := json.Unmarshal(row.Config, &config); err != nil {
			return seymour.Filter{}, fmt.Errorf("decode list filter config: %w", err)
		}
		filter.ListConfig = &config
	default:
		return seymour.Filter{}, fmt.Errorf("unknown filter type %q", row.Type)
	}
	if err := seymour.ValidateFilter(filter); err != nil {
		return seymour.Filter{}, fmt.Errorf("invalid stored filter: %w", err)
	}
	return filter, nil
}

func filterConfig(t seymour.FilterType, ai *seymour.AIConfig, list *seymour.ListConfig) ([]byte, error) {
	switch t {
	case seymour.FilterTypeAI:
		return json.Marshal(ai)
	case seymour.FilterTypeList:
		return json.Marshal(list)
	default:
		return nil, fmt.Errorf("unknown filter type %q", t)
	}
}

func filterInternalError(action string, err error) error {
	return seymour.E(fmt.Errorf("%s: %w", action, err))
}

// Filters returns all filters belonging to userID, including disabled filters.
func (r Repo) Filters(ctx context.Context, userID string) ([]seymour.Filter, error) {
	const q = `SELECT id, user_id, type, enabled, config, created_at, updated_at FROM filters WHERE user_id = ? ORDER BY created_at, id;`
	var rows []filterRow
	if err := r.db.SelectContext(ctx, &rows, q, userID); err != nil {
		return nil, filterInternalError("fetch filters", err)
	}
	filters := make([]seymour.Filter, 0, len(rows))
	for _, row := range rows {
		filter, err := row.model()
		if err != nil {
			return nil, filterInternalError("decode filter", err)
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

// Filter returns a filter only when it belongs to userID.
func (r Repo) Filter(ctx context.Context, userID, id string) (seymour.Filter, error) {
	const q = `SELECT id, user_id, type, enabled, config, created_at, updated_at FROM filters WHERE user_id = ? AND id = ?;`
	var row filterRow
	if err := r.db.GetContext(ctx, &row, q, userID, id); errors.Is(err, sql.ErrNoRows) {
		return seymour.Filter{}, seymour.ErrNotFound
	} else if err != nil {
		return seymour.Filter{}, filterInternalError("fetch filter", err)
	}
	filter, err := row.model()
	if err != nil {
		return seymour.Filter{}, filterInternalError("decode filter", err)
	}
	return filter, nil
}

// CreateFilter creates a filter owned by userID.
func (r Repo) CreateFilter(ctx context.Context, userID string, args seymour.CreateFilterArgs) (seymour.Filter, error) {
	filter := seymour.Filter{UserID: userID, Type: args.Type, Enabled: args.Enabled, AIConfig: args.AIConfig, ListConfig: args.ListConfig}
	if err := seymour.ValidateFilter(filter); err != nil {
		return seymour.Filter{}, err
	}
	config, err := filterConfig(args.Type, args.AIConfig, args.ListConfig)
	if err != nil {
		return seymour.Filter{}, filterInternalError("encode filter config", err)
	}
	filter.ID = uuid.NewString() + filterNamespace
	const q = `INSERT INTO filters (id, user_id, type, enabled, config) VALUES (?, ?, ?, ?, ?);`
	if _, err = r.db.ExecContext(ctx, q, filter.ID, userID, args.Type, args.Enabled, config); err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return seymour.Filter{}, seymour.ErrConflict
		}
		return seymour.Filter{}, filterInternalError("insert filter", err)
	}
	return r.Filter(ctx, userID, filter.ID)
}

// UpdateFilter fully replaces a user's filter configuration and enabled state.
func (r Repo) UpdateFilter(ctx context.Context, userID, id string, args seymour.UpdateFilterArgs) (seymour.Filter, error) {
	current, err := r.Filter(ctx, userID, id)
	if err != nil {
		return seymour.Filter{}, err
	}
	filter := seymour.Filter{ID: id, UserID: userID, Type: current.Type, Enabled: args.Enabled, AIConfig: args.AIConfig, ListConfig: args.ListConfig}
	if err := seymour.ValidateFilter(filter); err != nil {
		return seymour.Filter{}, err
	}
	config, err := filterConfig(current.Type, args.AIConfig, args.ListConfig)
	if err != nil {
		return seymour.Filter{}, filterInternalError("encode filter config", err)
	}
	const q = `UPDATE filters SET enabled = ?, config = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND id = ?;`
	if _, err := r.db.ExecContext(ctx, q, args.Enabled, config, userID, id); err != nil {
		return seymour.Filter{}, filterInternalError("update filter", err)
	}
	return r.Filter(ctx, userID, id)
}

// DeleteFilter deletes a filter only when it belongs to userID.
func (r Repo) DeleteFilter(ctx context.Context, userID, id string) error {
	const q = `DELETE FROM filters WHERE user_id = ? AND id = ?;`
	result, err := r.db.ExecContext(ctx, q, userID, id)
	if err != nil {
		return filterInternalError("delete filter", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return filterInternalError("check deleted filter", err)
	}
	if count == 0 {
		return seymour.ErrNotFound
	}
	return nil
}
