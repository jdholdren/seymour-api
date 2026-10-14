package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
)

type filterServiceFake struct {
	filters       map[string]seymour.Filter
	createErr     error
	getErr        error
	updateErr     error
	deleteErr     error
	lastUserID    string
	lastFilterID  string
	lastCreate    seymour.CreateFilterArgs
	lastUpdate    seymour.UpdateFilterArgs
}

func (f *filterServiceFake) Filters(_ context.Context, userID string) ([]seymour.Filter, error) {
	f.lastUserID = userID
	var result []seymour.Filter
	for _, filter := range f.filters {
		if filter.UserID == userID {
			result = append(result, filter)
		}
	}
	return result, f.getErr
}

func (f *filterServiceFake) Filter(_ context.Context, userID, id string) (seymour.Filter, error) {
	f.lastUserID, f.lastFilterID = userID, id
	if f.getErr != nil {
		return seymour.Filter{}, f.getErr
	}
	filter, ok := f.filters[id]
	if !ok || filter.UserID != userID {
		return seymour.Filter{}, seymour.ErrNotFound
	}
	return filter, nil
}

func (f *filterServiceFake) CreateFilter(_ context.Context, userID string, args seymour.CreateFilterArgs) (seymour.Filter, error) {
	f.lastUserID, f.lastCreate = userID, args
	if f.createErr != nil {
		return seymour.Filter{}, f.createErr
	}
	filter := seymour.Filter{ID: "created", UserID: userID, Type: args.Type, Enabled: args.Enabled, AIConfig: args.AIConfig, ListConfig: args.ListConfig}
	if f.filters == nil {
		f.filters = make(map[string]seymour.Filter)
	}
	f.filters[filter.ID] = filter
	return filter, nil
}

func (f *filterServiceFake) UpdateFilter(_ context.Context, userID, id string, args seymour.UpdateFilterArgs) (seymour.Filter, error) {
	f.lastUserID, f.lastFilterID, f.lastUpdate = userID, id, args
	if f.updateErr != nil {
		return seymour.Filter{}, f.updateErr
	}
	filter, ok := f.filters[id]
	if !ok || filter.UserID != userID {
		return seymour.Filter{}, seymour.ErrNotFound
	}
	filter.Enabled, filter.AIConfig, filter.ListConfig = args.Enabled, args.AIConfig, args.ListConfig
	f.filters[id] = filter
	return filter, nil
}

func (f *filterServiceFake) DeleteFilter(_ context.Context, userID, id string) error {
	f.lastUserID, f.lastFilterID = userID, id
	if f.deleteErr != nil {
		return f.deleteErr
	}
	filter, ok := f.filters[id]
	if !ok || filter.UserID != userID {
		return seymour.ErrNotFound
	}
	delete(f.filters, id)
	return nil
}

func filterServer(t *testing.T, service *filterServiceFake) (*Server, http.Handler) {
	t.Helper()
	key := []byte("01234567890123456789012345678901")
	server := NewServer(0, "", nil, service, nil, nil, nil, nil, key, nil, nil)
	return server, server.Handler
}

func filterRequest(t *testing.T, server *Server, handler http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != "" {
		value, err := server.secureCookie.Encode(sessionCookie, session{UserID: userID})
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestFilterRoutesReturnAIAndListConfigurations(t *testing.T) {
	// Both configuration variants use the real router and a valid signed session.
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	service := &filterServiceFake{filters: map[string]seymour.Filter{
		"ai-id": {ID: "ai-id", UserID: "owner", Type: seymour.FilterTypeAI, Enabled: true, AIConfig: &seymour.AIConfig{Prompt: "prompt"}, CreatedAt: createdAt, UpdatedAt: createdAt},
		"list-id": {ID: "list-id", UserID: "owner", Type: seymour.FilterTypeList, Enabled: false, ListConfig: &seymour.ListConfig{Allowlist: []string{"one"}, Disallowlist: []string{"two"}}, CreatedAt: createdAt, UpdatedAt: createdAt},
	}}
	server, handler := filterServer(t, service)

	// The list envelope stays present and nested arrays serialize as empty arrays rather than null.
	response := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters", "", "owner")
	if response.Code != http.StatusOK {
		t.Fatalf("GET filters status = %d, want %d: %s", response.Code, http.StatusOK, response.Body)
	}
	var list apiv1.FilterListResp
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Filters) != 2 {
		t.Fatalf("GET filters returned %d filters, want 2", len(list.Filters))
	}
	// Flat responses retain identity/config fields without exposing extras.
	var rawList struct {
		Filters []map[string]json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rawList); err != nil {
		t.Fatal(err)
	}
	for _, item := range rawList.Filters {
		for _, key := range []string{"id", "user_id", "type", "enabled", "created_at", "updated_at"} {
			if _, ok := item[key]; !ok {
				t.Errorf("filter response missing %q: %s", key, response.Body)
			}
		}
		if _, ai := item["ai_config"]; ai {
			if _, list := item["list_config"]; list {
				t.Errorf("filter response included both config variants: %s", response.Body)
			}
		}
		if len(item) != 7 {
			t.Errorf("filter response has unexpected fields: %s", response.Body)
		}
	}
	var listItem *apiv1.FilterResp
	for i := range list.Filters {
		if list.Filters[i].ID == "list-id" {
			listItem = &list.Filters[i]
		}
	}
	if listItem == nil || listItem.ListConfig == nil || listItem.ListConfig.Allowlist == nil || listItem.ListConfig.Disallowlist == nil {
		t.Fatalf("list configuration or its arrays were null: %+v", listItem)
	}

	// Single-filter reads retain the flat response fields and matching config variant.
	response = filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters/ai-id", "", "owner")
	var ai apiv1.FilterResp
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &ai) != nil || ai.ID != "ai-id" || ai.UserID != "owner" || ai.Type != "ai" || ai.AIConfig == nil || ai.AIConfig.Prompt != "prompt" || ai.ListConfig != nil || !ai.CreatedAt.Equal(createdAt) {
		t.Fatalf("GET AI filter response = %d %s", response.Code, response.Body)
	}
	response = filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters/list-id", "", "owner")
	var listFilter apiv1.FilterResp
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &listFilter) != nil || listFilter.Type != "list" || listFilter.ListConfig == nil || listFilter.AIConfig != nil || len(listFilter.ListConfig.Allowlist) != 1 || len(listFilter.ListConfig.Disallowlist) != 1 {
		t.Fatalf("GET list filter response = %d %s", response.Code, response.Body)
	}
}

func TestFilterRouteRejectsOmittedListConfigurationOnReplacement(t *testing.T) {
	// Omitting a list configuration is not interpreted as retaining or clearing the existing value.
	service := &filterServiceFake{filters: map[string]seymour.Filter{"list": {ID: "list", UserID: "owner", Type: seymour.FilterTypeList, ListConfig: &seymour.ListConfig{Allowlist: []string{"old"}}}}}
	server, handler := filterServer(t, service)

	// A full replacement without its required variant is rejected, leaving stored configuration intact.
	response := filterRequest(t, server, handler, http.MethodPut, "/api/users/owner/filters/list", `{"enabled":false}`, "owner")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("PUT status = %d, want 400: %s", response.Code, response.Body)
	}
	stored := service.filters["list"]
	if stored.ListConfig == nil || len(stored.ListConfig.Allowlist) != 1 || stored.ListConfig.Allowlist[0] != "old" {
		t.Fatalf("omitted configuration changed stored filter: %+v", stored)
	}
}

func TestFilterRoutesAuthenticateAndScopeEveryMethod(t *testing.T) {
	// Every registered method requires a session and refuses a path owned by another user.
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/users/owner/filters"},
		{name: "create", method: http.MethodPost, path: "/api/users/owner/filters", body: `{"type":"ai","enabled":true,"ai_config":{"prompt":"x"}}`},
		{name: "get", method: http.MethodGet, path: "/api/users/owner/filters/id"},
		{name: "update", method: http.MethodPut, path: "/api/users/owner/filters/id", body: `{"enabled":true,"ai_config":{"prompt":"x"}}`},
		{name: "delete", method: http.MethodDelete, path: "/api/users/owner/filters/id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := &filterServiceFake{filters: map[string]seymour.Filter{"id": {ID: "id", UserID: "owner", Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "x"}}}}
			server, handler := filterServer(t, service)

			// Without a signed session, the router rejects each operation before the fake service observes it.
			response := filterRequest(t, server, handler, tc.method, tc.path, tc.body, "")
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated status = %d, want 401", response.Code)
			}

			// A valid session for another user cannot use this resource path.
			otherPath := strings.Replace(tc.path, "/owner/", "/other/", 1)
			response = filterRequest(t, server, handler, tc.method, otherPath, tc.body, "owner")
			if response.Code != http.StatusForbidden {
				t.Fatalf("mismatched-owner status = %d, want 403", response.Code)
			}
			if service.lastUserID != "" {
				t.Fatalf("unauthorized request reached service for %q", service.lastUserID)
			}
		})
	}
}

func TestFilterRoutesReturnEmptyListAndCreateBothVariants(t *testing.T) {
	// An empty collection is represented by an empty filters array.
	service := &filterServiceFake{filters: map[string]seymour.Filter{}}
	server, handler := filterServer(t, service)
	response := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters", "", "owner")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"filters":[]`) {
		t.Fatalf("empty GET response = %d %s", response.Code, response.Body)
	}

	// Valid AI and list bodies reach the service with explicit false preserved.
	for _, body := range []string{
		`{"type":"ai","enabled":false,"ai_config":{"prompt":"prompt"}}`,
		`{"type":"list","enabled":true,"list_config":{"allowlist":[],"disallowlist":["blocked"]}}`,
	} {
		response = filterRequest(t, server, handler, http.MethodPost, "/api/users/owner/filters", body, "owner")
		if response.Code != http.StatusCreated {
			t.Fatalf("POST body %s status = %d: %s", body, response.Code, response.Body)
		}
		if service.lastUserID != "owner" {
			t.Fatalf("create user = %q, want owner", service.lastUserID)
		}
		if strings.Contains(body, `"type":"ai"`) && service.lastCreate.Enabled {
			t.Fatalf("explicit false was not preserved: %+v", service.lastCreate)
		}
	}
	if service.lastCreate.Type != seymour.FilterTypeList || service.lastCreate.ListConfig == nil || service.lastCreate.Enabled != true {
		t.Fatalf("list create arguments = %+v", service.lastCreate)
	}
}

func TestFilterRouteRejectsInvalidPostAndPutConfigurations(t *testing.T) {
	// Invalid oneOf configurations and attempted type changes share the same bad-request contract.
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "post both variants", method: http.MethodPost, path: "/api/users/owner/filters", body: `{"type":"ai","enabled":true,"ai_config":{"prompt":"ok"},"list_config":{"allowlist":[]}}`},
		{name: "post list type with AI config", method: http.MethodPost, path: "/api/users/owner/filters", body: `{"type":"list","enabled":true,"ai_config":{"prompt":"ok"}}`},
		{name: "put both variants", method: http.MethodPut, path: "/api/users/owner/filters/ai", body: `{"enabled":true,"ai_config":{"prompt":"ok"},"list_config":{"allowlist":[]}}`},
		{name: "put wrong variant", method: http.MethodPut, path: "/api/users/owner/filters/ai", body: `{"enabled":true,"list_config":{"allowlist":[]}}`},
		{name: "put type mutation", method: http.MethodPut, path: "/api/users/owner/filters/ai", body: `{"type":"list","enabled":true,"ai_config":{"prompt":"ok"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := &filterServiceFake{filters: map[string]seymour.Filter{"ai": {ID: "ai", UserID: "owner", Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "old"}}}}
			server, handler := filterServer(t, service)

			// The invalid request must fail before a create/update is persisted.
			response := filterRequest(t, server, handler, tc.method, tc.path, tc.body, "owner")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("response status = %d, want 400: %s", response.Code, response.Body)
			}
			if service.lastCreate.Type != "" || service.lastUpdate.AIConfig != nil || service.lastUpdate.ListConfig != nil {
				t.Fatalf("invalid configuration reached mutation: create=%+v update=%+v", service.lastCreate, service.lastUpdate)
			}
		})
	}
}

func TestFilterRouteRejectsMissingOrNullPutEnabled(t *testing.T) {
	// Full replacement requires enabled to be present and non-null.
	for _, body := range []string{`{"ai_config":{"prompt":"ok"}}`, `{"enabled":null,"ai_config":{"prompt":"ok"}}`} {
		t.Run(body, func(t *testing.T) {
			service := &filterServiceFake{filters: map[string]seymour.Filter{"ai": {ID: "ai", UserID: "owner", Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "old"}}}}
			server, handler := filterServer(t, service)

			// Invalid enabled presence is rejected before fetching or updating the resource.
			response := filterRequest(t, server, handler, http.MethodPut, "/api/users/owner/filters/ai", body, "owner")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("PUT status = %d, want 400: %s", response.Code, response.Body)
			}
			if service.lastUpdate.AIConfig != nil {
				t.Fatalf("invalid PUT reached update: %+v", service.lastUpdate)
			}
		})
	}
}

func TestFilterRoutesRejectInvalidRequestBodies(t *testing.T) {
	// Required enabled presence, config validation, and strict JSON decoding are enforced before persistence.
	service := &filterServiceFake{filters: map[string]seymour.Filter{}}
	server, handler := filterServer(t, service)
	for _, body := range []string{
		`{"type":"ai","ai_config":{"prompt":"prompt"}}`,
		`{"type":"ai","enabled":null,"ai_config":{"prompt":"prompt"}}`,
		`{"type":"ai","enabled":true}`,
		`{"type":"ai","enabled":true,"ai_config":{"prompt":" "}}`,
		`{"type":"list","enabled":true,"list_config":{"allowlist":[" "]}}`,
		`{"type":"unknown","enabled":true,"ai_config":{"prompt":"prompt"}}`,
		`{"type":"ai","enabled":true,"ai_config":{"prompt":"ok"},"extra":1}`,
		`{"type":"ai","enabled":true,"ai_config":{"prompt":"ok"}} {}`,
		`null`,
	} {
		response := filterRequest(t, server, handler, http.MethodPost, "/api/users/owner/filters", body, "owner")
		if response.Code != http.StatusBadRequest {
			t.Errorf("POST body %s status = %d, want 400: %s", body, response.Code, response.Body)
		}
	}
	if service.lastCreate.Type != "" {
		t.Fatalf("invalid bodies reached CreateFilter: %+v", service.lastCreate)
	}
}

func TestFilterRoutesRequireAuthenticationAndMatchingUser(t *testing.T) {
	// Authentication is checked by the real router before service methods run.
	service := &filterServiceFake{filters: map[string]seymour.Filter{}}
	server, handler := filterServer(t, service)
	response := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET status = %d, want 401", response.Code)
	}

	// A session cannot address another user's collection or item.
	response = filterRequest(t, server, handler, http.MethodGet, "/api/users/other/filters", "", "owner")
	if response.Code != http.StatusForbidden {
		t.Fatalf("mismatched user GET status = %d, want 403", response.Code)
	}
	if service.lastUserID != "" {
		t.Fatalf("mismatched user reached service as %q", service.lastUserID)
	}
}

func TestFilterRoutesMapMissingForeignAndServiceErrors(t *testing.T) {
	// Foreign and missing records share the same scoped not-found behavior.
	service := &filterServiceFake{filters: map[string]seymour.Filter{"foreign": {ID: "foreign", UserID: "other", Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "x"}}}}
	server, handler := filterServer(t, service)
	for _, id := range []string{"missing", "foreign"} {
		response := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters/"+id, "", "owner")
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", id, response.Code)
		}
		response = filterRequest(t, server, handler, http.MethodPut, "/api/users/owner/filters/"+id, `{"enabled":true,"ai_config":{"prompt":"x"}}`, "owner")
		if response.Code != http.StatusNotFound {
			t.Errorf("PUT %s status = %d, want 404", id, response.Code)
		}
		response = filterRequest(t, server, handler, http.MethodDelete, "/api/users/owner/filters/"+id, "", "owner")
		if response.Code != http.StatusNotFound {
			t.Errorf("DELETE %s status = %d, want 404", id, response.Code)
		}
	}

	// Unclassified service failures are hidden behind an internal server response.
	service.getErr = errors.New("database detail")
	response := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters", "", "owner")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("service error status = %d, want 500", response.Code)
	}
}

func TestFilterRoutesReplaceConfigurationAndDelete(t *testing.T) {
	// PUT fully replaces list values, clearing previous entries while preserving explicit false.
	service := &filterServiceFake{filters: map[string]seymour.Filter{"one": {ID: "one", UserID: "owner", Type: seymour.FilterTypeList, Enabled: true, ListConfig: &seymour.ListConfig{Allowlist: []string{"old"}}}}}
	server, handler := filterServer(t, service)
	response := filterRequest(t, server, handler, http.MethodPut, "/api/users/owner/filters/one", `{"enabled":false,"list_config":{"allowlist":[],"disallowlist":[]}}`, "owner")
	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d: %s", response.Code, response.Body)
	}
	updated := service.filters["one"]
	if updated.Enabled || updated.AIConfig != nil || updated.ListConfig == nil || len(updated.ListConfig.Allowlist) != 0 || len(updated.ListConfig.Disallowlist) != 0 {
		t.Fatalf("PUT did not replace fields: %+v", updated)
	}

	// Deletion responds without a JSON body.
	response = filterRequest(t, server, handler, http.MethodDelete, "/api/users/owner/filters/one", "", "owner")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("DELETE response = %d %q", response.Code, response.Body.String())
	}
}

func TestFilterRoutesConflictAndInternalErrors(t *testing.T) {
	// Duplicate creation errors preserve their conflict status.
	service := &filterServiceFake{createErr: seymour.ErrConflict}
	server, handler := filterServer(t, service)
	response := filterRequest(t, server, handler, http.MethodPost, "/api/users/owner/filters", `{"type":"ai","enabled":true,"ai_config":{"prompt":"x"}}`, "owner")
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate POST status = %d, want 409", response.Code)
	}

	// An untyped service failure maps to 500 for mutations as well as reads.
	service.createErr = errors.New("internal failure")
	response = filterRequest(t, server, handler, http.MethodPost, "/api/users/owner/filters", `{"type":"ai","enabled":true,"ai_config":{"prompt":"x"}}`, "owner")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("internal POST status = %d, want 500", response.Code)
	}
}
