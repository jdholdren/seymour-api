package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/securecookie"
	"github.com/jdholdren/seymour/internal/seymour"
)

type timelinePromptUsers struct {
	seymour.UserService
	prompt   *string
	userErr  error
	setErr   error
	setCalls int
}

func (u *timelinePromptUsers) User(_ context.Context, id string) (seymour.User, error) {
	if u.userErr != nil {
		return seymour.User{}, u.userErr
	}

	return seymour.User{ID: id, TimelinePrompt: u.prompt}, nil
}

func (u *timelinePromptUsers) UpdateUser(_ context.Context, _ string, args seymour.UpdateUserArgs) error {
	u.setCalls++
	if u.setErr != nil {
		return u.setErr
	}

	if args.TimelinePrompt != nil {
		if *args.TimelinePrompt == "" {
			u.prompt = nil
		} else {
			u.prompt = args.TimelinePrompt
		}
	}

	return nil
}

func newTimelinePromptAPI(t *testing.T) (http.Handler, *securecookie.SecureCookie, *timelinePromptUsers) {
	t.Helper()

	users := &timelinePromptUsers{}
	hashKey := []byte("01234567890123456789012345678901")
	blockKey := []byte("0123456789012345")
	secure := securecookie.New(hashKey, blockKey)

	frontend, _ := url.Parse("http://localhost:3000")
	server := NewServer(0, "", nil, nil, users, nil, nil, hashKey, blockKey, frontend)

	return server.Handler, secure, users
}

func timelinePromptRequest(t *testing.T, handler http.Handler, secure *securecookie.SecureCookie, method, userID, body string, authenticated bool) *httptest.ResponseRecorder {
	return timelinePromptRequestAs(t, handler, secure, method, userID, userID, body, authenticated)
}

func timelinePromptRequestAs(t *testing.T, handler http.Handler, secure *securecookie.SecureCookie, method, userID, sessionID, body string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, "/api/users/"+userID+"/timeline-prompt", strings.NewReader(body))
	if authenticated {
		value, err := secure.Encode(sessionCookie, session{UserID: sessionID})
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	return resp
}

func TestTimelinePromptAuthAndRoundTrip(t *testing.T) {
	handler, secure, users := newTimelinePromptAPI(t)
	initial := "existing prompt"
	users.prompt = &initial

	if got := timelinePromptRequest(t, handler, secure, http.MethodGet, "alice", "", false).Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", got)
	}
	if got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", `{"prompt":"unauthorized"}`, false).Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated PUT status = %d, want 401", got)
	}
	if got := timelinePromptRequestAs(t, handler, secure, http.MethodPut, "alice", "bob", `{"prompt":"cross-user"}`, true).Code; got != http.StatusForbidden {
		t.Fatalf("cross-user status = %d, want 403", got)
	}
	if users.setCalls != 0 || users.prompt == nil || *users.prompt != initial {
		t.Fatalf("unauthorized PUT mutated prompt: calls=%d prompt=%v", users.setCalls, users.prompt)
	}

	got := timelinePromptRequest(t, handler, secure, http.MethodGet, "alice", "", true)
	var response struct {
		Prompt string `json:"prompt"`
	}
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &response) != nil || response.Prompt != initial {
		t.Fatalf("GET after rejected PUT = %d %s", got.Code, got.Body.String())
	}

	prompt := "Keep  spaces and \"quotes\"\n世界"
	got = timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", fmt.Sprintf(`{"prompt":%q}`, prompt), true)
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &response) != nil || response.Prompt != prompt {
		t.Fatalf("PUT = %d %s", got.Code, got.Body.String())
	}
	if users.prompt == nil || *users.prompt != prompt {
		t.Fatalf("stored prompt = %v", users.prompt)
	}

	got = timelinePromptRequest(t, handler, secure, http.MethodGet, "alice", "", true)
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &response) != nil || response.Prompt != prompt {
		t.Fatalf("GET after PUT = %d %s", got.Code, got.Body.String())
	}

	got = timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", `{"prompt":""}`, true)
	if got.Code != http.StatusOK || users.prompt != nil {
		t.Fatalf("clear PUT = %d %s, stored %v", got.Code, got.Body.String(), users.prompt)
	}

	got = timelinePromptRequest(t, handler, secure, http.MethodGet, "alice", "", true)
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &response) != nil || response.Prompt != "" {
		t.Fatalf("GET after clear = %d %s", got.Code, got.Body.String())
	}
}

func TestTimelinePromptServiceErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		method     string
		userErr    error
		setErr     error
		wantStatus int
	}{
		{"GET not found", http.MethodGet, seymour.ErrNotFound, nil, http.StatusNotFound},
		{"PUT not found", http.MethodPut, nil, seymour.ErrNotFound, http.StatusNotFound},
		{"GET typed internal error", http.MethodGet, seymour.E("user lookup failed", http.StatusInternalServerError), nil, http.StatusInternalServerError},
		{"PUT typed internal error", http.MethodPut, nil, seymour.E("prompt update failed", http.StatusInternalServerError), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, secure, users := newTimelinePromptAPI(t)
			users.userErr = tc.userErr
			users.setErr = tc.setErr

			got := timelinePromptRequest(t, handler, secure, tc.method, "alice", `{"prompt":"new"}`, true)
			if got.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", got.Code, tc.wantStatus, got.Body.String())
			}
		})
	}
}

func TestTimelinePromptRejectsInvalidBodies(t *testing.T) {
	handler, secure, _ := newTimelinePromptAPI(t)

	for _, body := range []string{
		`{"prompt":1}`, `{"prompt":false}`, `{"prompt":[]}`, `[]`,
		`{"prompt":"x"} {}`, `{"prompt":"unterminated}`, "not json",
	} {
		got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", body, true)
		if got.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, got.Code)
		}
	}
}

func TestTimelinePromptEmptyValuesClear(t *testing.T) {
	for _, body := range []string{`{"prompt":""}`, `{}`, `{"prompt":null}`} {
		t.Run(body, func(t *testing.T) {
			handler, secure, users := newTimelinePromptAPI(t)
			prompt := "existing prompt"
			users.prompt = &prompt

			got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", body, true)
			if got.Code != http.StatusOK || users.prompt != nil {
				t.Fatalf("clear PUT = %d %s, stored %v", got.Code, got.Body.String(), users.prompt)
			}
		})
	}
}

func TestTimelinePromptDoesNotLimitBodySize(t *testing.T) {
	handler, secure, users := newTimelinePromptAPI(t)
	body := strings.Repeat(" ", 128*1024) + `{"prompt":"keep this"}`

	got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", body, true)
	if got.Code != http.StatusOK || users.prompt == nil || *users.prompt != "keep this" {
		t.Fatalf("PUT with large body = %d %s, stored %v", got.Code, got.Body.String(), users.prompt)
	}
}

func TestTimelinePromptSizeLimits(t *testing.T) {
	handler, secure, _ := newTimelinePromptAPI(t)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"decoded limit", fmt.Sprintf(`{"prompt":%q}`, strings.Repeat("a", maxTimelinePromptBytes+1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", tc.body, true)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", got.Code)
			}
		})
	}

	// Escaped JSON may exceed the decoded prompt length.
	got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", fmt.Sprintf(`{"prompt":%q}`, strings.Repeat("\n", maxTimelinePromptBytes/2)), true)
	if got.Code != http.StatusOK {
		t.Fatalf("escaped prompt status = %d, want 200", got.Code)
	}

	for _, tc := range []struct {
		name   string
		prompt string
		status int
	}{
		{"exact byte boundary", strings.Repeat("a", maxTimelinePromptBytes), http.StatusOK},
		{"multibyte byte overflow", strings.Repeat("a", maxTimelinePromptBytes-1) + "é", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := timelinePromptRequest(t, handler, secure, http.MethodPut, "alice", fmt.Sprintf(`{"prompt":%q}`, tc.prompt), true)
			if got.Code != tc.status {
				t.Fatalf("status = %d, want %d", got.Code, tc.status)
			}
		})
	}
}
