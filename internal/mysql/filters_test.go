package mysql_test

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdholdren/seymour/internal/seymour"
)

func TestCreateFilterRoundTripsConfigurations(t *testing.T) {
	// Prepare both configurations with content that must survive JSON encoding unchanged.
	repo := testRepo(t)
	ctx := t.Context()
	aiConfig := &seymour.AIConfig{Prompt: "Quote: \"hello\"; café 🐈\nsecond line"}
	listConfig := &seymour.ListConfig{
		Allowlist:    []string{"quoted \"value\"", "日本語\nline"},
		Disallowlist: []string{"β"},
	}

	// Create both supported types for the same owner, with different enabled states.
	ai, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: aiConfig})
	require.NoError(t, err)
	list, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeList, ListConfig: listConfig})
	require.NoError(t, err)

	// Created rows have namespaced IDs, nonzero timestamps, and preserve enabled state.
	for _, filter := range []seymour.Filter{ai, list} {
		assert.True(t, strings.HasSuffix(filter.ID, "-fl"))
		assert.False(t, filter.CreatedAt.IsZero())
		assert.False(t, filter.UpdatedAt.IsZero())
	}

	// Config content and enabled state are independent of storage-generated metadata.
	assert.True(t, ai.Enabled)
	assert.False(t, list.Enabled)
	assert.Equal(t, aiConfig, ai.AIConfig)
	assert.Nil(t, ai.ListConfig)
	assert.Equal(t, listConfig, list.ListConfig)
	assert.Nil(t, list.AIConfig)
}

func TestFiltersListIsUserScopedAndOrdered(t *testing.T) {
	// Same-user filters of different types coexist, as do identical types for different users.
	repo := testRepo(t)
	ctx := t.Context()
	list, err := repo.CreateFilter(ctx, "user-a", seymour.CreateFilterArgs{Type: seymour.FilterTypeList, ListConfig: &seymour.ListConfig{Allowlist: []string{"one"}}})
	require.NoError(t, err)
	ai, err := repo.CreateFilter(ctx, "user-a", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "a"}})
	require.NoError(t, err)
	other, err := repo.CreateFilter(ctx, "user-b", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "b"}})
	require.NoError(t, err)

	// List each tenant and a user who has not configured any filters.
	filters, err := repo.Filters(ctx, "user-a")
	require.NoError(t, err)
	otherFilters, err := repo.Filters(ctx, "user-b")
	require.NoError(t, err)
	empty, err := repo.Filters(ctx, "nobody")
	require.NoError(t, err)

	// The owner's rows follow creation time and ID, independently of enabled state.
	require.Len(t, filters, 2)
	expected := []seymour.Filter{list, ai}
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].CreatedAt.Equal(expected[j].CreatedAt) {
			return expected[i].ID < expected[j].ID
		}
		return expected[i].CreatedAt.Before(expected[j].CreatedAt)
	})
	assert.Equal(t, []string{expected[0].ID, expected[1].ID}, []string{filters[0].ID, filters[1].ID})

	// Each listing contains exactly its user's filters, including disabled configurations.
	assert.ElementsMatch(t, expected, filters)
	assert.Equal(t, []seymour.Filter{other}, otherFilters)
	assert.Empty(t, empty)
}

func TestFilterReadUpdateDeleteAreUserScoped(t *testing.T) {
	// Create a filter whose owner is distinct from the caller attempting unauthorized access.
	repo := testRepo(t)
	ctx := t.Context()
	created, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "original"}})
	require.NoError(t, err)

	// The owner can read the complete configuration.
	found, err := repo.Filter(ctx, "owner", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)

	// Another tenant's read receives the same not-found response as an absent filter.
	_, err = repo.Filter(ctx, "intruder", created.ID)
	assertNotFound(t, err)

	// Unauthorized replacement and deletion cannot reach the owner's row.
	_, err = repo.UpdateFilter(ctx, "intruder", created.ID, seymour.UpdateFilterArgs{AIConfig: &seymour.AIConfig{Prompt: "stolen"}})
	assertNotFound(t, err)
	assertNotFound(t, repo.DeleteFilter(ctx, "intruder", created.ID))

	// Unauthorized writes leave the entire owner's filter unchanged.
	found, err = repo.Filter(ctx, "owner", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)
}

func TestDeleteFilterRemovesOwnedRow(t *testing.T) {
	// Prepare an owned filter that can be deleted without affecting another tenant.
	repo := testRepo(t)
	ctx := t.Context()
	created, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "original"}})
	require.NoError(t, err)

	// Delete the owned row once.
	require.NoError(t, repo.DeleteFilter(ctx, "owner", created.ID))

	// Subsequent reads and mutations report not-found instead of recreating the filter.
	_, err = repo.UpdateFilter(ctx, "owner", "missing-fl", seymour.UpdateFilterArgs{AIConfig: &seymour.AIConfig{Prompt: "x"}})
	assertNotFound(t, err)
	assertNotFound(t, repo.DeleteFilter(ctx, "owner", "missing-fl"))

	// The deleted identity remains absent for reads, replacements, and repeated deletion.
	_, err = repo.Filter(ctx, "owner", created.ID)
	assertNotFound(t, err)
	_, err = repo.UpdateFilter(ctx, "owner", created.ID, seymour.UpdateFilterArgs{AIConfig: &seymour.AIConfig{Prompt: "x"}})
	assertNotFound(t, err)
	assertNotFound(t, repo.DeleteFilter(ctx, "owner", created.ID))
}

func TestUpdateAIFilterFullyReplacesAndAdvancesTimestamp(t *testing.T) {
	// A known old timestamp makes updated_at advancement deterministic without sleeping.
	repo := testRepo(t)
	ctx := t.Context()
	created, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "before"}})
	require.NoError(t, err)
	past := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	_, err = testDB.ExecContext(ctx, `UPDATE filters SET created_at = ?, updated_at = ? WHERE id = ?`, past, past, created.ID)
	require.NoError(t, err)

	// Replace the prompt and disable the existing filter in a single update.
	updated, err := repo.UpdateFilter(ctx, "owner", created.ID, seymour.UpdateFilterArgs{Enabled: false, AIConfig: &seymour.AIConfig{Prompt: "after"}})
	require.NoError(t, err)

	// Replacement preserves immutable fields and advances the update timestamp.
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, created.UserID, updated.UserID)
	assert.Equal(t, created.Type, updated.Type)
	assert.False(t, updated.Enabled)
	assert.Equal(t, "after", updated.AIConfig.Prompt)
	assert.True(t, updated.CreatedAt.Equal(past))
	assert.True(t, updated.UpdatedAt.After(past))
}

func TestUpdateFilterUnchangedReplacementSucceeds(t *testing.T) {
	// Prepare a disabled filter whose current values will be submitted unchanged.
	repo := testRepo(t)
	ctx := t.Context()
	created, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "unchanged"}})
	require.NoError(t, err)

	// Repeat exactly the same full replacement, even when MySQL reports no changed rows.
	updated, err := repo.UpdateFilter(ctx, "owner", created.ID, seymour.UpdateFilterArgs{AIConfig: created.AIConfig})

	// An existing row is not mistaken for a missing filter.
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, created.AIConfig, updated.AIConfig)
	assert.False(t, updated.Enabled)
}

func TestUpdateListFilterClearsReplacedLists(t *testing.T) {
	// A list filter begins with both lists populated before a full replacement.
	repo := testRepo(t)
	created, err := repo.CreateFilter(t.Context(), "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeList, Enabled: true, ListConfig: &seymour.ListConfig{Allowlist: []string{"allow"}, Disallowlist: []string{"deny"}}})
	require.NoError(t, err)

	// Replacement uses only the new allowlist and clears the old disallowlist.
	updated, err := repo.UpdateFilter(t.Context(), "owner", created.ID, seymour.UpdateFilterArgs{ListConfig: &seymour.ListConfig{Allowlist: []string{"replacement"}}})

	// Omitted old list entries are not merged into the replacement configuration.
	require.NoError(t, err)
	assert.Equal(t, []string{"replacement"}, updated.ListConfig.Allowlist)
	assert.Empty(t, updated.ListConfig.Disallowlist)
}

func TestCreateFilterInvalidConfigurationDoesNotInsert(t *testing.T) {
	// Prepare invalid type/config combinations for a user with no stored filters.
	repo := testRepo(t)
	ctx := t.Context()
	inputs := []seymour.CreateFilterArgs{
		{Type: "unknown", AIConfig: &seymour.AIConfig{Prompt: "prompt"}},
		{Type: seymour.FilterTypeAI},
		{Type: seymour.FilterTypeAI, ListConfig: &seymour.ListConfig{}},
		{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "prompt"}, ListConfig: &seymour.ListConfig{}},
		{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: " \n"}},
		{Type: seymour.FilterTypeList, ListConfig: &seymour.ListConfig{Allowlist: []string{" "}}},
	}

	// Invalid AI/list configurations are rejected as bad requests without inserting rows.
	for _, input := range inputs {
		_, err := repo.CreateFilter(ctx, "owner", input)
		assertStatus(t, err, http.StatusBadRequest)
	}

	// Failed creations leave the user's listing empty.
	filters, err := repo.Filters(ctx, "owner")
	require.NoError(t, err)
	assert.Empty(t, filters)
}

func TestCreateFilterDisabledDuplicateConflicts(t *testing.T) {
	// A disabled row still occupies the unique user/type slot.
	repo := testRepo(t)
	ctx := t.Context()
	created, err := repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "valid"}})
	require.NoError(t, err)

	// Attempt to create an enabled filter of the same type for the same user.
	_, err = repo.CreateFilter(ctx, "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "duplicate"}})

	// Uniqueness rejects the duplicate without replacing the disabled configuration.
	assertConflict(t, err)
	found, err := repo.Filter(ctx, "owner", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)
}

func TestUpdateFilterInvalidReplacementPreservesRow(t *testing.T) {
	// Capture the complete row before submitting an invalid replacement.
	repo := testRepo(t)
	created, err := repo.CreateFilter(t.Context(), "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "original"}})
	require.NoError(t, err)

	// Rejected replacement does not change any persisted field.
	_, err = repo.UpdateFilter(t.Context(), "owner", created.ID, seymour.UpdateFilterArgs{Enabled: false, AIConfig: &seymour.AIConfig{Prompt: ""}})

	// Validation returns a bad request and leaves the previous row unchanged.
	assertStatus(t, err, http.StatusBadRequest)
	found, err := repo.Filter(t.Context(), "owner", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)
}

func TestCreateFilterConcurrentDuplicateHasSingleWinner(t *testing.T) {
	// Prepare concurrent callers sharing one unique user/type slot.
	repo := testRepo(t)
	const workers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	// The common barrier makes all goroutines race the same user/type insert.
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.CreateFilter(context.Background(), "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "same"}})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	// Exactly one insert succeeds, every other attempt conflicts, and one row persists.
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if statusOf(err) == http.StatusConflict {
			conflicts++
		} else {
			t.Errorf("unexpected create error: %v", err)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, workers-1, conflicts)
	filters, err := repo.Filters(context.Background(), "owner")
	require.NoError(t, err)
	assert.Len(t, filters, 1)
}

func TestFiltersRejectCorruptedStoredConfiguration(t *testing.T) {
	// Store valid JSON with an invalid prompt type, bypassing application validation.
	repo := testRepo(t)
	created, err := repo.CreateFilter(t.Context(), "owner", seymour.CreateFilterArgs{Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "valid"}})
	require.NoError(t, err)
	_, err = testDB.ExecContext(t.Context(), `UPDATE filters SET config = ? WHERE id = ?`, `{"prompt":42}`, created.ID)
	require.NoError(t, err)

	// A single-row lookup rejects configuration that cannot be decoded into the declared type.
	_, err = repo.Filter(t.Context(), "owner", created.ID)
	assertStatus(t, err, http.StatusInternalServerError)

	// Collection reads also fail rather than returning a partially decoded listing.
	_, err = repo.Filters(t.Context(), "owner")
	assertStatus(t, err, http.StatusInternalServerError)
}

func statusOf(err error) int {
	var domainErr *seymour.Error
	if !errors.As(err, &domainErr) {
		return 0
	}
	return domainErr.Status
}

func assertStatus(t *testing.T, err error, status int) {
	t.Helper()
	var domainErr *seymour.Error
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, status, domainErr.Status)
}

func assertNotFound(t *testing.T, err error) { assertStatus(t, err, http.StatusNotFound) }
func assertConflict(t *testing.T, err error) { assertStatus(t, err, http.StatusConflict) }
