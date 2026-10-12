package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jdholdren/seymour/internal/mock"
	"github.com/jdholdren/seymour/internal/seymour"
)

type rig struct {
	server   Server
	feeds    *mock.MockFeedService
	timeline *mock.MockTimelineService
	users    *mock.MockUserService
}

func newRig(t *testing.T) *rig {
	t.Helper()

	ctrl := gomock.NewController(t)
	r := &rig{
		feeds:    mock.NewMockFeedService(ctrl),
		timeline: mock.NewMockTimelineService(ctrl),
		users:    mock.NewMockUserService(ctrl),
	}
	r.server = Server{feeds: r.feeds, timeline: r.timeline, users: r.users}

	return r
}

func TestGetTimelinePromptReturnsStoredPrompt(t *testing.T) {
	prompt := "Show me science news"
	rig := newRig(t)
	rig.users.EXPECT().User(gomock.Any(), "alice").Return(seymour.User{TimelinePrompt: &prompt}, nil)

	req := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodGet, "/", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
	recorder := httptest.NewRecorder()

	err := rig.server.getTimelinePrompt(recorder, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"prompt":"Show me science news"}`, recorder.Body.String())
}

func TestGetTimelinePromptReturnsEmptyStringWhenUnset(t *testing.T) {
	rig := newRig(t)
	rig.users.EXPECT().User(gomock.Any(), "alice").Return(seymour.User{}, nil)

	req := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodGet, "/", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
	recorder := httptest.NewRecorder()

	err := rig.server.getTimelinePrompt(recorder, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"prompt":""}`, recorder.Body.String())
}

func TestPutTimelinePromptUpdatesPromptWithoutChangingText(t *testing.T) {
	prompt, body := "  first line\n雪 ☃  \nsecond line ", `{"prompt":"  first line\n雪 ☃  \nsecond line "}`
	rig := newRig(t)
	rig.users.EXPECT().UpdateUser(gomock.Any(), "alice", seymour.UpdateUserArgs{TimelinePrompt: &prompt}).Return(nil)

	r := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodPut, "/api/users/alice/timeline-prompt", strings.NewReader(body))
	r = mux.SetURLVars(r, map[string]string{"userID": "alice"})
	w := httptest.NewRecorder()

	err := rig.server.putTimelinePrompt(w, r)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, body, w.Body.String())
}

func TestPutTimelinePromptClearsPromptWhenEmpty(t *testing.T) {
	prompt := ""
	body := `{"prompt":""}`
	rig := newRig(t)
	rig.users.EXPECT().UpdateUser(gomock.Any(), "alice", seymour.UpdateUserArgs{TimelinePrompt: &prompt}).Return(nil)

	r := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodPut, "/api/users/alice/timeline-prompt", strings.NewReader(body))
	r = mux.SetURLVars(r, map[string]string{"userID": "alice"})
	w := httptest.NewRecorder()

	err := rig.server.putTimelinePrompt(w, r)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, body, w.Body.String())
}

func TestPutTimelinePromptAccepts1000UnicodeCharacters(t *testing.T) {
	prompt := strings.Repeat("é", 1000)
	body := `{"prompt":"` + prompt + `"}`
	rig := newRig(t)
	rig.users.EXPECT().UpdateUser(gomock.Any(), "alice", seymour.UpdateUserArgs{TimelinePrompt: &prompt}).Return(nil)

	r := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodPut, "/api/users/alice/timeline-prompt", strings.NewReader(body))
	r = mux.SetURLVars(r, map[string]string{"userID": "alice"})
	w := httptest.NewRecorder()

	err := rig.server.putTimelinePrompt(w, r)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, body, w.Body.String())
}

func TestPutTimelinePromptRejectsMoreThan1000ASCIICharacters(t *testing.T) {
	rig := newRig(t)
	ctx := context.WithValue(t.Context(), userIDCtxKey, "alice")
	body := `{"prompt":"` + strings.Repeat("a", 1001) + `"}`
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/users/alice/timeline-prompt", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
	recorder := httptest.NewRecorder()

	err := rig.server.putTimelinePrompt(recorder, req)

	var apiErr *seymour.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestPutTimelinePromptRejectsMoreThan1000UnicodeCharacters(t *testing.T) {
	rig := newRig(t)
	ctx := context.WithValue(t.Context(), userIDCtxKey, "alice")
	body := `{"prompt":"` + strings.Repeat("界", 1001) + `"}`
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/users/alice/timeline-prompt", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
	recorder := httptest.NewRecorder()

	err := rig.server.putTimelinePrompt(recorder, req)

	var apiErr *seymour.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestGetTimelinePromptReturnsUnauthorizedWithoutUserContext(t *testing.T) {
	rig := newRig(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/users/alice/timeline-prompt", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})

	err := rig.server.getTimelinePrompt(httptest.NewRecorder(), req)

	var apiErr *seymour.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestGetTimelinePromptReturnsForbiddenForDifferentUser(t *testing.T) {
	rig := newRig(t)
	ctx := context.WithValue(t.Context(), userIDCtxKey, "bob")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/users/alice/timeline-prompt", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})

	err := rig.server.getTimelinePrompt(httptest.NewRecorder(), req)

	var apiErr *seymour.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusForbidden, apiErr.Status)
}

func TestPutTimelinePromptReturnsUnauthorizedWithoutUserContext(t *testing.T) {
	rig := newRig(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/users/alice/timeline-prompt", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})

	err := rig.server.putTimelinePrompt(httptest.NewRecorder(), req)

	var apiErr *seymour.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestPutTimelinePromptReturnsForbiddenForDifferentUser(t *testing.T) {
	rig := newRig(t)
	ctx := context.WithValue(t.Context(), userIDCtxKey, "bob")
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/users/alice/timeline-prompt", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "alice"})

	err := rig.server.putTimelinePrompt(httptest.NewRecorder(), req)

	var apiErr *seymour.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusForbidden, apiErr.Status)
}
