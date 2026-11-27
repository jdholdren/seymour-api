package mysql_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/mysql"
	"github.com/jdholdren/seymour/internal/seymour"
)

func testWorkQueueRepo(t *testing.T) mysql.Repo {
	t.Helper()

	// Queue tests isolate only their own table, even if unrelated schema tables are unavailable.
	t.Cleanup(func() {
		_, err := testDB.ExecContext(context.Background(), "TRUNCATE TABLE work_items;")
		require.NoError(t, err)
	})
	return mysql.New(testDB)
}

func TestWorkQueueEnqueueConflictAndReenqueueAfterCompletion(t *testing.T) {
	// A repeated incomplete type/key pair must produce a Seymour conflict.
	repo := testWorkQueueRepo(t)
	ctx := context.Background()
	input := json.RawMessage(`{"feed_id":"feed-1"}`)
	first, err := repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, "feed-1", input)
	require.NoError(t, err)
	require.Equal(t, int64(1), first.FencingGeneration)
	require.NotNil(t, first.Input)
	require.JSONEq(t, string(input), string(*first.Input))

	// The same key remains available in a distinct work type.
	otherType, err := repo.EnqueueWork(ctx, seymour.WorkType("other"), "feed-1", nil)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, otherType.ID)

	_, err = repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, "feed-1", nil)
	var serviceErr *seymour.Error
	require.ErrorAs(t, err, &serviceErr)
	require.Equal(t, 409, serviceErr.Status)

	// Completing the original item permits another row with the same type and key.
	leased, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.NoError(t, err)
	result := json.RawMessage(`{"synced":true}`)
	require.NoError(t, repo.CompleteWork(ctx, leased.ID, leased.FencingGeneration, result))
	repeated, err := repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, "feed-1", nil)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, repeated.ID)

	// Completion stores its optional result alongside the completion timestamp.
	var storedResult *json.RawMessage
	require.NoError(t, testDB.GetContext(ctx, &storedResult, `SELECT result FROM work_items WHERE id = ?;`, first.ID))
	require.NotNil(t, storedResult)
	require.JSONEq(t, string(result), string(*storedResult))
}

func TestWorkQueueLeaseAndFencing(t *testing.T) {
	// The first lease uses generation one and blocks another claim while still live.
	repo := testWorkQueueRepo(t)
	ctx := context.Background()
	_, err := repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, "feed-lease", nil)
	require.NoError(t, err)
	firstLease, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.NoError(t, err)
	require.Equal(t, int64(1), firstLease.FencingGeneration)
	require.NotNil(t, firstLease.LeasedAt)
	require.NotNil(t, firstLease.LeasedUntil)

	_, err = repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.ErrorIs(t, err, seymour.ErrNotFound)

	// Reclaiming an expired lease advances its fence so the former holder cannot complete it.
	_, err = testDB.ExecContext(ctx, `UPDATE work_items SET leased_until = ? WHERE id = ?;`, time.Now().Add(-time.Minute), firstLease.ID)
	require.NoError(t, err)
	secondLease, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.NoError(t, err)
	require.Equal(t, firstLease.FencingGeneration+1, secondLease.FencingGeneration)
	require.ErrorIs(t, repo.CompleteWork(ctx, firstLease.ID, firstLease.FencingGeneration, nil), seymour.ErrConflict)
	require.NoError(t, repo.CompleteWork(ctx, secondLease.ID, secondLease.FencingGeneration, nil))
}

func TestWorkQueueLeaseMultipleTypes(t *testing.T) {
	// Workers can restrict claims to a subset, then claim any remaining type with an empty filter.
	repo := testWorkQueueRepo(t)
	ctx := context.Background()
	_, err := repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, "feed-filtered", nil)
	require.NoError(t, err)
	_, err = repo.EnqueueWork(ctx, seymour.WorkType("other"), "other-filtered", nil)
	require.NoError(t, err)

	subsetLease, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkType("other")}, time.Minute)
	require.NoError(t, err)
	require.Equal(t, seymour.WorkType("other"), subsetLease.Type)

	allTypesLease, err := repo.LeaseWork(ctx, nil, time.Minute)
	require.NoError(t, err)
	require.Equal(t, seymour.WorkTypeFeedSync, allTypesLease.Type)
}

func TestWorkQueueReleaseExpiredWork(t *testing.T) {
	// Expiry cleanup clears lease data and advances the fence for stale workers.
	repo := testWorkQueueRepo(t)
	ctx := context.Background()
	_, err := repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, "feed-release", nil)
	require.NoError(t, err)
	lease, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.NoError(t, err)
	_, err = testDB.ExecContext(ctx, `UPDATE work_items SET leased_until = ? WHERE id = ?;`, time.Now().Add(-time.Minute), lease.ID)
	require.NoError(t, err)

	count, err := repo.ReleaseExpiredWork(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.ErrorIs(t, repo.CompleteWork(ctx, lease.ID, lease.FencingGeneration, nil), seymour.ErrConflict)
	released, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.NoError(t, err)
	require.Equal(t, lease.FencingGeneration+1, released.FencingGeneration)
}

func TestWorkQueueLeaseSkipsLockedItem(t *testing.T) {
	// Two eligible items make it possible to verify that the locked row is skipped.
	repo := testWorkQueueRepo(t)
	ctx := context.Background()
	for _, key := range []string{"feed-a", "feed-b"} {
		_, err := repo.EnqueueWork(ctx, seymour.WorkTypeFeedSync, key, nil)
		require.NoError(t, err)
	}

	// Holding a row lock in another transaction deterministically forces the lease query to exercise SKIP LOCKED.
	tx, err := testDB.BeginTxx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var lockedID string
	err = tx.GetContext(ctx, &lockedID, `SELECT id FROM work_items
		WHERE type = ? AND completed_at IS NULL
		ORDER BY id LIMIT 1 FOR UPDATE;`, seymour.WorkTypeFeedSync)
	require.NoError(t, err)

	// A claim made while the first row is locked must lease the other eligible item.
	leased, err := repo.LeaseWork(ctx, []seymour.WorkType{seymour.WorkTypeFeedSync}, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, lockedID, leased.ID)
}
