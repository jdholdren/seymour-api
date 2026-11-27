package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/jdholdren/seymour/internal/seymour"
)

const workItemNamespace = "-wrk"

func (r Repo) EnqueueWork(ctx context.Context, workType seymour.WorkType, key string, input json.RawMessage) (seymour.WorkItem, error) {
	if len(input) > 0 && !json.Valid(input) {
		return seymour.WorkItem{}, seymour.E("work item input must be valid JSON", 400)
	}

	// Insert only the identity fields; database defaults initialize lease state and generation.
	item := seymour.WorkItem{
		ID:    uuid.NewString() + workItemNamespace,
		Type:  workType,
		Key:   key,
		Input: optionalJSON(input),
	}
	const q = "INSERT INTO work_items (id, type, `key`, input) VALUES (?, ?, ?, ?);"
	if _, err := r.db.ExecContext(ctx, q, item.ID, item.Type, item.Key, item.Input); err != nil {
		if mysqlErr := (&mysqldriver.MySQLError{}); errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return seymour.WorkItem{}, seymour.E("incomplete work item already exists", 409)
		}
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error enqueueing work item: %w", err), 500)
	}

	// Read back database defaults so the returned model reflects persisted state.
	const selectQ = "SELECT id, type, `key`, input, result, leased_at, leased_until, fencing_generation, completed_at FROM work_items WHERE id = ?;"
	if err := r.db.GetContext(ctx, &item, selectQ, item.ID); err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error fetching enqueued work item: %w", err), 500)
	}
	return item, nil
}

func (r Repo) LeaseWork(ctx context.Context, workTypes []seymour.WorkType, leaseDuration time.Duration) (seymour.WorkItem, error) {
	if leaseDuration <= 0 {
		return seymour.WorkItem{}, seymour.E("lease duration must be positive", 400)
	}

	// Lock one eligible row so concurrent workers cannot claim the same lease.
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error beginning work lease transaction: %w", err), 500)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	var item seymour.WorkItem
	// Empty type lists intentionally omit the type predicate, letting workers consume all queue types.
	query := sq.Select("id", "type", "`key`", "input", "result", "leased_at", "leased_until", "fencing_generation", "completed_at").
		From("work_items").
		Where(sq.Eq{"completed_at": nil}).
		Where(sq.Or{
			sq.Eq{"leased_until": nil},
			sq.LtOrEq{"leased_until": now},
		}).
		OrderBy("id").
		Limit(1).
		Suffix("FOR UPDATE SKIP LOCKED")
	if len(workTypes) > 0 {
		query = query.Where(sq.Eq{"type": workTypes})
	}
	selectQ, args, err := query.ToSql()
	if err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error constructing work lease query: %w", err), 500)
	}
	err = tx.GetContext(ctx, &item, selectQ, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return seymour.WorkItem{}, seymour.ErrNotFound
	}
	if err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error selecting work item to lease: %w", err), 500)
	}

	// Increment only when stealing an existing expired lease; the first lease uses generation one.
	leasedUntil := now.Add(leaseDuration)
	const updateQ = `UPDATE work_items SET
		fencing_generation = fencing_generation + IF(leased_until IS NULL, 0, 1),
		leased_at = ?, leased_until = ?
		WHERE id = ?;`
	if _, err := tx.ExecContext(ctx, updateQ, now, leasedUntil, item.ID); err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error updating work item lease: %w", err), 500)
	}
	const fetchQ = "SELECT id, type, `key`, input, result, leased_at, leased_until, fencing_generation, completed_at FROM work_items WHERE id = ?;"
	if err := tx.GetContext(ctx, &item, fetchQ, item.ID); err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error fetching leased work item: %w", err), 500)
	}
	if err := tx.Commit(); err != nil {
		return seymour.WorkItem{}, seymour.E(fmt.Errorf("error committing work item lease: %w", err), 500)
	}
	return item, nil
}

func (r Repo) CompleteWork(ctx context.Context, id string, fencingGeneration int64, result json.RawMessage) error {
	if len(result) > 0 && !json.Valid(result) {
		return seymour.E("work item result must be valid JSON", 400)
	}

	// A completion is valid only for the current, still-live lease.
	now := time.Now().UTC()
	const q = `UPDATE work_items SET result = ?, completed_at = ?
		WHERE id = ? AND fencing_generation = ? AND completed_at IS NULL
		AND leased_until > ?;`
	dbResult, err := r.db.ExecContext(ctx, q, result, now, id, fencingGeneration, now)
	if err != nil {
		return seymour.E(fmt.Errorf("error completing work item: %w", err), 500)
	}

	affected, err := dbResult.RowsAffected()
	if err != nil {
		return seymour.E(fmt.Errorf("error checking work completion result: %w", err), 500)
	}
	if affected == 0 {
		return seymour.ErrConflict
	}

	return nil
}

func optionalJSON(value json.RawMessage) *json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return &value
}

func (r Repo) ReleaseExpiredWork(ctx context.Context) (int64, error) {
	// Clearing expired lease fields makes incomplete rows claimable; generation changes fence old workers.
	const q = `UPDATE work_items SET
		leased_at = NULL, leased_until = NULL,
		fencing_generation = fencing_generation + 1
		WHERE completed_at IS NULL AND leased_until <= ?;`
	result, err := r.db.ExecContext(ctx, q, time.Now().UTC())
	if err != nil {
		return 0, seymour.E(fmt.Errorf("error releasing expired work items: %w", err), 500)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, seymour.E(fmt.Errorf("error checking released work item count: %w", err), 500)
	}

	return affected, nil
}
