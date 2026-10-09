package api

import (
	"context"
	"encoding/json"
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

func TestGetTimelinePrompt(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{name: "stored prompt", prompt: "Show me science news"},
		{name: "unset prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stored *string
			if tt.prompt != "" {
				stored = &tt.prompt
			}

			users := mock.NewMockUserService(gomock.NewController(t))
			users.EXPECT().User(gomock.Any(), "alice").Return(seymour.User{TimelinePrompt: stored}, nil)

			req := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodGet, "/", nil)
			req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
			recorder := httptest.NewRecorder()

			err := (Server{users: users}).getTimelinePrompt(recorder, req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Prompt string `json:"prompt"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, tt.prompt, response.Prompt)
		})
	}
}

func TestPutTimelinePrompt(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		prompt string
	}{
		{name: "ordinary prompt", body: `{"prompt":"  keep spacing\n雪 ☃  "}`, prompt: "  keep spacing\n雪 ☃  "},
		{name: "empty string clears", body: `{"prompt":""}`},
		{name: "1000 Unicode characters accepted", body: `{"prompt":"` + strings.Repeat("é", 1000) + `"}`, prompt: strings.Repeat("é", 1000)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := mock.NewMockUserService(gomock.NewController(t))
			users.EXPECT().UpdateUser(gomock.Any(), "alice", seymour.UpdateUserArgs{TimelinePrompt: &tc.prompt}).Return(nil)

			r := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDCtxKey, "alice"), http.MethodPut, "/api/users/alice/timeline-prompt", strings.NewReader(tc.body))
			r = mux.SetURLVars(r, map[string]string{"userID": "alice"})
			w := httptest.NewRecorder()
			err := (Server{users: users}).putTimelinePrompt(w, r)

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, w.Code)
			var response struct {
				Prompt string `json:"prompt"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Equal(t, tc.prompt, response.Prompt)
		})
	}
}

func TestTimelinePromptValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "over 1000 ASCII characters", body: `{"prompt":"` + strings.Repeat("a", 1001) + `"}`},
		{name: "over 1000 Unicode characters", body: `{"prompt":"` + strings.Repeat("界", 1001) + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mock.NewMockUserService(gomock.NewController(t))
			ctx := context.WithValue(t.Context(), userIDCtxKey, "alice")
			req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/", strings.NewReader(tt.body))
			req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
			recorder := httptest.NewRecorder()

			err := (Server{users: users}).putTimelinePrompt(recorder, req)

			var apiErr *seymour.Error
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})
	}
}

func TestTimelinePromptAuth(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		handler     func(Server, http.ResponseWriter, *http.Request) error
		contextUser string
		wantStatus  int
	}{
		{"GET missing user", http.MethodGet, Server.getTimelinePrompt, "", http.StatusUnauthorized},
		{"GET mismatched user", http.MethodGet, Server.getTimelinePrompt, "bob", http.StatusForbidden},
		{"PUT missing user", http.MethodPut, Server.putTimelinePrompt, "", http.StatusUnauthorized},
		{"PUT mismatched user", http.MethodPut, Server.putTimelinePrompt, "bob", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.contextUser != "" {
				ctx = context.WithValue(ctx, userIDCtxKey, tt.contextUser)
			}
			req := httptest.NewRequestWithContext(ctx, tt.method, "/", strings.NewReader(`{"prompt":""}`))
			req = mux.SetURLVars(req, map[string]string{"userID": "alice"})
			rec := httptest.NewRecorder()
			server := Server{users: mock.NewMockUserService(gomock.NewController(t))}

			err := tt.handler(server, rec, req)

			var apiErr *seymour.Error
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, tt.wantStatus, apiErr.Status)
		})
	}
}
