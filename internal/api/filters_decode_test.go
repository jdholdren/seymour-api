package api

import (
	"net/http"
	"testing"
)

func TestFilterRoutesDecodeJSONWhitespace(t *testing.T) {
	// JSON whitespace belongs to the decoder; non-JSON whitespace and non-object bodies stay invalid.
	const body = `{"type":"ai","enabled":false,"ai_config":{"prompt":"prompt"}}`
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{name: "JSON whitespace", body: " \t\r\n" + body + " \t\r\n", status: http.StatusCreated},
		{name: "Unicode whitespace", body: "\u00a0" + body, status: http.StatusBadRequest},
		{name: "empty body", status: http.StatusBadRequest},
		{name: "whitespace only", body: " \t\r\n", status: http.StatusBadRequest},
		{name: "array", body: "[]", status: http.StatusBadRequest},
		{name: "null", body: "null", status: http.StatusBadRequest},
		{name: "second value", body: body + " {}", status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each request uses an authenticated owner and a fresh service to observe persistence.
			service := &filterServiceFake{}
			server, handler := filterServer(t, service)

			// Submit the unchanged bytes through the real route and its decoder.
			response := filterRequest(t, server, handler, http.MethodPost, "/api/users/owner/filters", tc.body, "owner")

			// Only valid JSON reaches creation; required-field validation still rejects null.
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.status, response.Body)
			}
			if tc.status == http.StatusBadRequest && service.lastCreate.Type != "" {
				t.Fatal("invalid body reached CreateFilter")
			}
		})
	}
}
