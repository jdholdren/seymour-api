package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
)

type filterServiceFake struct {
	filters map[string]seymour.Filter
}

func (f *filterServiceFake) Filters(_ context.Context, userID string) ([]seymour.Filter, error) {
	var result []seymour.Filter
	for _, filter := range f.filters {
		if filter.UserID == userID {
			result = append(result, filter)
		}
	}
	return result, nil
}

func (f *filterServiceFake) Filter(_ context.Context, userID, id string) (seymour.Filter, error) {
	filter, ok := f.filters[id]
	if !ok || filter.UserID != userID {
		return seymour.Filter{}, seymour.ErrNotFound
	}
	return filter, nil
}

func (f *filterServiceFake) CreateFilter(_ context.Context, userID string, args seymour.CreateFilterArgs) (seymour.Filter, error) {
	filter := seymour.Filter{ID: "created", UserID: userID, Type: args.Type, Enabled: args.Enabled, AIConfig: args.AIConfig, ListConfig: args.ListConfig}
	if f.filters == nil {
		f.filters = make(map[string]seymour.Filter)
	}
	f.filters[filter.ID] = filter
	return filter, nil
}

func (f *filterServiceFake) UpdateFilter(ctx context.Context, userID, id string, args seymour.UpdateFilterArgs) (seymour.Filter, error) {
	filter, err := f.Filter(ctx, userID, id)
	if err != nil {
		return seymour.Filter{}, err
	}
	filter.Enabled, filter.AIConfig, filter.ListConfig = args.Enabled, args.AIConfig, args.ListConfig
	f.filters[id] = filter
	return filter, nil
}

func (f *filterServiceFake) DeleteFilter(ctx context.Context, userID, id string) error {
	if _, err := f.Filter(ctx, userID, id); err != nil {
		return err
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

// TODO: Set session information on the request context in handler tests so setup
// can avoid signing cookies; keep middleware authentication coverage separate.
func filterRequest(t *testing.T, server *Server, handler http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	value, err := server.secureCookie.Encode(sessionCookie, session{UserID: userID})
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestFilterRoutesCreateAndRead(t *testing.T) {
	// Each supported configuration follows the same creation and read contract.
	cases := []struct {
		name string
		body string
		want apiv1.FilterResp
	}{
		{name: "ai", body: `{"type":"ai","enabled":false,"ai_config":{"prompt":"prompt"}}`, want: apiv1.FilterResp{ID: "created", UserID: "owner", Type: "ai", AIConfig: &apiv1.FilterAIConfig{Prompt: "prompt"}}},
		{name: "list", body: `{"type":"list","enabled":true,"list_config":{"allowlist":["Go"],"disallowlist":["ads"]}}`, want: apiv1.FilterResp{ID: "created", UserID: "owner", Type: "list", Enabled: true, ListConfig: &apiv1.FilterListConfig{Allowlist: []string{"Go"}, Disallowlist: []string{"ads"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Use the registered routes and an authenticated owner with empty storage.
			server, handler := filterServer(t, &filterServiceFake{})

			// Creating a filter returns its assigned identity and complete configuration.
			created := filterRequest(t, server, handler, http.MethodPost, "/api/users/owner/filters", tc.body, "owner")
			if created.Code != http.StatusCreated {
				t.Fatalf("POST status = %d: %s", created.Code, created.Body)
			}
			var got apiv1.FilterResp
			if err := json.Unmarshal(created.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("created filter = %+v, want %+v", got, tc.want)
			}

			// Reading the new resource returns the same response as creation.
			read := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters/created", "", "owner")
			if read.Code != http.StatusOK || read.Body.String() != created.Body.String() {
				t.Fatalf("GET response = %d %s", read.Code, read.Body)
			}
		})
	}
}

func TestFilterRoutesList(t *testing.T) {
	// A user's collection can be empty or contain both supported filter variants.
	for _, count := range []int{0, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			// Populate only the requested scenario with owner-scoped filters.
			service := &filterServiceFake{filters: map[string]seymour.Filter{}}
			if count > 0 {
				service.filters["ai"] = seymour.Filter{ID: "ai", UserID: "owner", Type: seymour.FilterTypeAI, AIConfig: &seymour.AIConfig{Prompt: "prompt"}}
				service.filters["list"] = seymour.Filter{ID: "list", UserID: "owner", Type: seymour.FilterTypeList, ListConfig: &seymour.ListConfig{}}
			}
			server, handler := filterServer(t, service)

			// Fetch the collection through its registered route.
			response := filterRequest(t, server, handler, http.MethodGet, "/api/users/owner/filters", "", "owner")

			// The envelope always contains an array with the user's stored filters.
			var got apiv1.FilterListResp
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &got) != nil || got.Filters == nil || len(got.Filters) != count {
				t.Fatalf("GET collection = %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestFilterRoutesReplaceConfiguration(t *testing.T) {
	// A list filter starts enabled with values that the replacement must remove.
	service := &filterServiceFake{filters: map[string]seymour.Filter{"one": {ID: "one", UserID: "owner", Type: seymour.FilterTypeList, Enabled: true, ListConfig: &seymour.ListConfig{Allowlist: []string{"old"}, Disallowlist: []string{"ads"}}}}}
	server, handler := filterServer(t, service)

	// An empty list config replaces both lists rather than retaining old values.
	response := filterRequest(t, server, handler, http.MethodPut, "/api/users/owner/filters/one", `{"enabled":false,"list_config":{}}`, "owner")

	// The response preserves identity/type, applies false, and emits empty arrays.
	var got apiv1.FilterResp
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &got) != nil {
		t.Fatalf("PUT response = %d %s", response.Code, response.Body)
	}
	want := apiv1.FilterResp{ID: "one", UserID: "owner", Type: "list", ListConfig: &apiv1.FilterListConfig{Allowlist: []string{}, Disallowlist: []string{}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("replacement = %+v, want %+v", got, want)
	}
}

func TestFilterRoutesDelete(t *testing.T) {
	// The authenticated owner has a filter to delete.
	service := &filterServiceFake{filters: map[string]seymour.Filter{"one": {ID: "one", UserID: "owner"}}}
	server, handler := filterServer(t, service)

	// Delete the owned resource through the registered route.
	response := filterRequest(t, server, handler, http.MethodDelete, "/api/users/owner/filters/one", "", "owner")

	// Successful deletion removes the record and returns no response body.
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 || len(service.filters) != 0 {
		t.Fatalf("DELETE response = %d %q, remaining = %d", response.Code, response.Body.String(), len(service.filters))
	}
}
