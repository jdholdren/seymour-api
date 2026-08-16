package mysql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
)

func TestCreateFilter_AllowList(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	id, err := repo.CreateFilter(ctx, "user-1", seymour.AllowListConfig{
		Keywords: []string{"golang", "temporal"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	filters, err := repo.UserFilters(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, filters, 1)

	got, ok := filters[0].(seymour.AllowListConfig)
	require.True(t, ok)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "user-1", got.UserID)
	assert.ElementsMatch(t, []string{"golang", "temporal"}, got.Keywords)
}

func TestCreateFilter_DisallowList(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	id, err := repo.CreateFilter(ctx, "user-1", seymour.DisallowListConfig{
		Keywords: []string{"crypto"},
	})
	require.NoError(t, err)

	filters, err := repo.UserFilters(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, filters, 1)

	got, ok := filters[0].(seymour.DisallowListConfig)
	require.True(t, ok)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, []string{"crypto"}, got.Keywords)
}

func TestUserFilters_ScopedByUser(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	_, err := repo.CreateFilter(ctx, "user-1", seymour.AllowListConfig{Keywords: []string{"a"}})
	require.NoError(t, err)
	_, err = repo.CreateFilter(ctx, "user-2", seymour.AllowListConfig{Keywords: []string{"b"}})
	require.NoError(t, err)

	filters, err := repo.UserFilters(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, filters, 1)
	assert.Equal(t, []string{"a"}, filters[0].(seymour.AllowListConfig).Keywords)
}

func TestUserFilters_Empty(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	filters, err := repo.UserFilters(ctx, "user-1")
	require.NoError(t, err)
	assert.Empty(t, filters)
}

func TestCreateFilter_Webhook(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	id, err := repo.CreateFilter(ctx, "user-1", seymour.WebhookConfig{Host: "example.com"})
	require.NoError(t, err)

	filters, err := repo.UserFilters(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, filters, 1)

	got, ok := filters[0].(seymour.WebhookConfig)
	require.True(t, ok)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "example.com", got.Host)
}
