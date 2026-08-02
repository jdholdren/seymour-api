package sqlite_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/jdholdren/seymour/internal/migrations"
	"github.com/jdholdren/seymour/internal/seymour"
	"github.com/jdholdren/seymour/internal/sqlite"
)

func testRepo(t *testing.T) sqlite.Repo {
	t.Helper()

	db, err := sqlx.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, migrations.Run(db))

	return sqlite.New(db)
}

func TestEnsureUser_CreatesNewUser(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	user, login, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	assert.NotEmpty(t, user.ID)
	assert.Equal(t, user.ID, login.UserID)
	assert.Equal(t, seymour.Idp("github"), login.Idp)
	assert.Equal(t, "1234", login.IdpID)
}

func TestEnsureUser_ReturnsExistingUser(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

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
	ctx := context.Background()

	_, err := repo.User(ctx, "does-not-exist")
	require.Error(t, err)

	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestUser_Found(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	created, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "5678")
	require.NoError(t, err)

	found, err := repo.User(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}
