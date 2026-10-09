package mysql_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
)

func TestEnsureUser_CreatesNewUser(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	user, login, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	assert.NotEmpty(t, user.ID)
	assert.Equal(t, user.ID, login.UserID)
	assert.Equal(t, seymour.Idp("github"), login.Idp)
	assert.Equal(t, "1234", login.IdpID)
}

func TestEnsureUser_ReturnsExistingUser(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	user1, login1, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	user2, login2, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, login1.ID, login2.ID)
	assert.Equal(t, login1.LastLogin, login2.LastLogin)
}

func TestUser_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	_, err := repo.User(ctx, "does-not-exist")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestUser_Found(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()

	created, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "5678")
	require.NoError(t, err)

	found, err := repo.User(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Nil(t, found.TimelinePrompt)
}

func TestSetTimelinePrompt_PersistsAndClears(t *testing.T) {
	repo := testRepo(t)
	ctx := t.Context()
	user, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "prompt-user")
	require.NoError(t, err)
	const oldUpdatedAt = "2000-01-01 00:00:00"
	_, err = testDB.ExecContext(ctx, `UPDATE users SET updated_at = ? WHERE id = ?;`, oldUpdatedAt, user.ID)
	require.NoError(t, err)
	other, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "other-user")
	require.NoError(t, err)

	prompt := "Résumé 🌱\n第二行\n"
	require.NoError(t, repo.SetTimelinePrompt(ctx, user.ID, prompt))
	require.NoError(t, repo.SetTimelinePrompt(ctx, user.ID, prompt)) // identical writes still succeed
	found, err := repo.User(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, found.TimelinePrompt)
	assert.Equal(t, prompt, *found.TimelinePrompt)
	assert.True(t, found.UpdatedAt.After(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)))

	ensured, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "prompt-user")
	require.NoError(t, err)
	assert.Equal(t, user.ID, ensured.ID)
	require.NotNil(t, ensured.TimelinePrompt)
	assert.Equal(t, prompt, *ensured.TimelinePrompt)

	otherFound, err := repo.User(ctx, other.ID)
	require.NoError(t, err)
	assert.Nil(t, otherFound.TimelinePrompt)

	require.NoError(t, repo.SetTimelinePrompt(ctx, user.ID, ""))
	require.NoError(t, repo.SetTimelinePrompt(ctx, user.ID, ""))
	found, err = repo.User(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, found.TimelinePrompt)

	ensured, _, err = repo.EnsureUser(ctx, seymour.Idp("github"), "prompt-user")
	require.NoError(t, err)
	assert.Equal(t, user.ID, ensured.ID)
	found, err = repo.User(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, found.TimelinePrompt)
}

func TestSetTimelinePrompt_MissingUser(t *testing.T) {
	repo := testRepo(t)
	for _, prompt := range []string{"prompt", ""} {
		err := repo.SetTimelinePrompt(t.Context(), "does-not-exist", prompt)
		var sErr *seymour.Error
		require.ErrorAs(t, err, &sErr)
		assert.Equal(t, http.StatusNotFound, sErr.Status)
	}
}
