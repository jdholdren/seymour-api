package sqlite_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
)

func TestDeleteSubscription(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.CreateSubscription(ctx, "user-1", "feed-1"))

	subs, err := repo.AllSubscriptions(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)

	require.NoError(t, repo.DeleteSubscription(ctx, subs[0].ID))

	subs, err = repo.AllSubscriptions(ctx, "user-1")
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestAllSubscriptions_ScopedByUser(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.CreateSubscription(ctx, "user-1", "feed-1"))
	require.NoError(t, repo.CreateSubscription(ctx, "user-2", "feed-1"))

	subs, err := repo.AllSubscriptions(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "user-1", subs[0].UserID)
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	err := repo.DeleteSubscription(ctx, "does-not-exist")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}
