package mysql_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
)

func TestEnsureUser_CreatesNewUser(t *testing.T) {
	// Start with no user registered for this provider identity.
	repo := testRepo(t)
	ctx := t.Context()

	// First login creates a user and its provider login.
	user, login, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	// The login references the new user and supplied provider identity.
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, user.ID, login.UserID)
	assert.Equal(t, seymour.Idp("github"), login.Idp)
	assert.Equal(t, "1234", login.IdpID)
}

func TestEnsureUser_ReturnsExistingUser(t *testing.T) {
	// Register a user for the provider identity.
	repo := testRepo(t)
	ctx := t.Context()

	// First login establishes the user and login records.
	user1, login1, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	// Logging in again returns the existing records.
	user2, login2, err := repo.EnsureUser(ctx, seymour.Idp("github"), "1234")
	require.NoError(t, err)

	// Both calls identify the same user and login.
	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, login1.ID, login2.ID)
	assert.Equal(t, login1.LastLogin, login2.LastLogin)
}

func TestUser_NotFound(t *testing.T) {
	// Start with no matching user.
	repo := testRepo(t)
	ctx := t.Context()

	// Looking up an unknown ID fails.
	_, err := repo.User(ctx, "does-not-exist")
	require.Error(t, err)

	// The lookup reports the application's not-found status.
	var sErr *seymour.Error
	require.ErrorAs(t, err, &sErr)
	assert.Equal(t, http.StatusNotFound, sErr.Status)
}

func TestUser_Found(t *testing.T) {
	// Use an empty repository to create a known user.
	repo := testRepo(t)
	ctx := t.Context()

	// Register the user that will be retrieved by ID.
	created, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "5678")
	require.NoError(t, err)

	// Retrieval returns the registered user with no prompt set.
	found, err := repo.User(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Nil(t, found.TimelinePrompt)
}

func TestUpdateUserPersistsTimelinePrompt(t *testing.T) {
	// Prepare a repository for a user without a prompt.
	repo := testRepo(t)
	ctx := t.Context()

	// Register the user whose prompt will be updated.
	user, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "persist-prompt-user")
	require.NoError(t, err)

	// Save a prompt containing Unicode and line breaks.
	prompt := "Résumé 🌱\n第二行\n"
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{TimelinePrompt: &sql.NullString{String: prompt, Valid: true}}))

	// Reading the user returns the exact stored prompt.
	found, err := repo.User(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, found.TimelinePrompt)
	assert.Equal(t, prompt, *found.TimelinePrompt)
}

func TestUpdateUserUpdatesUserTimestamp(t *testing.T) {
	// Create a user whose update timestamp can be moved into the past.
	repo := testRepo(t)
	ctx := t.Context()
	user, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "timestamp-user")
	require.NoError(t, err)

	// Give the update a timestamp old enough to advance without sleeping.
	_, err = testDB.ExecContext(ctx, `UPDATE users SET updated_at = ? WHERE id = ?;`, "2000-01-01 00:00:00", user.ID)
	require.NoError(t, err)

	// Saving a prompt refreshes the user's update timestamp.
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{TimelinePrompt: &sql.NullString{String: "Updated", Valid: true}}))

	// The persisted timestamp is later than the previous value.
	found, err := repo.User(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, found.UpdatedAt.After(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)))
}

func TestUpdateUserClearsTimelinePrompt(t *testing.T) {
	// Create a user with a stored prompt to clear.
	repo := testRepo(t)
	ctx := t.Context()
	user, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "clear-prompt-user")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{TimelinePrompt: &sql.NullString{String: "Clear me", Valid: true}}))

	// An invalid NullString explicitly clears the stored prompt.
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{TimelinePrompt: &sql.NullString{}}))

	// Reading the user returns no prompt.
	found, err := repo.User(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, found.TimelinePrompt)
}

func TestUpdateUser_MissingUser(t *testing.T) {
	// Start with no user matching the update ID.
	repo := testRepo(t)

	// Both an empty update and a prompt update report a missing user.
	prompt := &sql.NullString{String: "prompt", Valid: true}
	for _, args := range []seymour.UpdateUserArgs{{}, {TimelinePrompt: prompt}} {
		err := repo.UpdateUser(t.Context(), "does-not-exist", args)

		// Each update returns the application's not-found status.
		var sErr *seymour.Error
		require.ErrorAs(t, err, &sErr)
		assert.Equal(t, http.StatusNotFound, sErr.Status)
	}
}

func TestUpdateUserUpdatesPreferredNameAndTimelinePrompt(t *testing.T) {
	// Create a user whose name and prompt will be set together.
	repo := testRepo(t)
	ctx := t.Context()
	user, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "partial-update-user")
	require.NoError(t, err)

	// Save both user fields in one update.
	name := &sql.NullString{String: "Ada", Valid: true}
	prompt := &sql.NullString{String: "Science news", Valid: true}
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{PreferredName: name, TimelinePrompt: prompt}))

	// Reading the user returns both supplied values.
	found, err := repo.User(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, found.PreferredName)
	assert.Equal(t, name.String, *found.PreferredName)
	require.NotNil(t, found.TimelinePrompt)
	assert.Equal(t, prompt.String, *found.TimelinePrompt)
}

func TestUpdateUser_StoresEmptyStringsAndClearsInvalidValues(t *testing.T) {
	// Create a user to exercise explicit empty and null values.
	repo := testRepo(t)
	ctx := t.Context()
	user, _, err := repo.EnsureUser(ctx, seymour.Idp("github"), "empty-update-user")
	require.NoError(t, err)

	// Valid empty strings are stored as values, not SQL NULL.
	empty := &sql.NullString{String: "", Valid: true}
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{PreferredName: empty, TimelinePrompt: empty}))

	// Both fields are present and contain empty strings.
	found, err := repo.User(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, found.PreferredName)
	assert.Equal(t, "", *found.PreferredName)
	require.NotNil(t, found.TimelinePrompt)
	assert.Equal(t, "", *found.TimelinePrompt)

	// Invalid values explicitly clear both fields to SQL NULL.
	cleared := &sql.NullString{}
	require.NoError(t, repo.UpdateUser(ctx, user.ID, seymour.UpdateUserArgs{PreferredName: cleared, TimelinePrompt: cleared}))

	// Both fields are now unset rather than empty strings.
	found, err = repo.User(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, found.PreferredName)
	assert.Nil(t, found.TimelinePrompt)
}
