// Command genopenapi generates the OpenAPI 3.0 spec for the Seymour API from
// the request/response types already defined in apis/v1, using reflection
// (github.com/swaggest/openapi-go). It's meant to be regenerated whenever the
// API's wire types or routes change (see `make gen-openapi`), and CI checks
// that the committed spec matches what this program produces.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"net/http"
	"os"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
)

// Doc-only structs describing query/path parameters. These mirror what the
// handlers in internal/api actually read off the request (see
// parsePaginationParams, mux.Vars, r.URL.Query()) but live here instead of in
// internal/api so the generator doesn't need to change handler behavior.
type (
	timelineQuery struct {
		FeedID string `query:"feed_id" description:"Filter the timeline to a single feed."`
		Status string `query:"status" description:"Filter by entry status (requires_judgement, approved, rejected). Defaults to all statuses."`
		From   string `query:"from" description:"Only include entries published on or after this date (RFC3339 or YYYY-MM-DD)."`
		To     string `query:"to" description:"Only include entries published on or before this date (RFC3339 or YYYY-MM-DD)."`
		Limit  int    `query:"limit" description:"Page size, default 20, max 100."`
		Offset int    `query:"offset" description:"Pagination offset, default 0."`
	}

	feedEntryPath struct {
		FeedEntryID string `path:"feedEntryID"`
	}

	oauthLoginQuery struct {
		RedirectPath string `query:"s" description:"Path on FRONTEND_URL to send the browser to after login. Defaults to /."`
	}

	oauthCallbackQuery struct {
		State string `query:"state" description:"Opaque state value echoed back from GitHub, verified against the oauth_state cookie."`
		Code  string `query:"code" description:"Authorization code to exchange for a GitHub access token."`
	}

	subscriptionPath struct {
		SubscriptionID string `path:"subscriptionID"`
	}

	filterUserPath struct {
		UserID string `path:"userID"`
	}

	filterPath struct {
		UserID   string `path:"userID"`
		FilterID string `path:"filterID"`
	}
)

func main() {
	out := flag.String("out", "openapi.yaml", "path to write the generated OpenAPI spec to")
	flag.Parse()

	reflector := openapi3.NewReflector()
	spec := reflector.SpecEns()
	spec.SetTitle("Seymour API")
	spec.SetDescription("REST API for Seymour, an RSS feed aggregator with a curated timeline.")
	spec.SetVersion("0.1.0")

	type op struct {
		method, path, id, summary, tag string
		reqBody, reqParams, resp       any
		status                         int
	}

	ops := []op{
		{
			method: http.MethodGet, path: "/api/viewer", id: "getViewer", tag: "viewer",
			summary: "Get info about the current viewer, including their subscriptions.",
			resp:    apiv1.Viewer{}, status: http.StatusOK,
		},
		{
			method: http.MethodPost, path: "/api/subscriptions", id: "createSubscription", tag: "subscriptions",
			summary: "Subscribe to a feed by URL, triggering the CreateFeed workflow.",
			reqBody: apiv1.PostSubscriptionReq{}, resp: apiv1.FeedResp{}, status: http.StatusCreated,
		},
		{
			method: http.MethodGet, path: "/api/subscriptions", id: "listSubscriptions", tag: "subscriptions",
			summary: "List the viewer's subscriptions.",
			resp:    apiv1.SubscriptionListResp{}, status: http.StatusCreated,
		},
		{
			method: http.MethodDelete, path: "/api/subscriptions/{subscriptionID}", id: "deleteSubscription", tag: "subscriptions",
			summary:   "Unsubscribe from a feed.",
			reqParams: subscriptionPath{}, status: http.StatusNoContent,
		},
		{
			method: http.MethodGet, path: "/api/users/{userID}/filters", id: "listFilters", tag: "filters",
			summary: "List the user's filters.", reqParams: filterUserPath{}, resp: apiv1.FilterListResp{}, status: http.StatusOK,
		},
		{
			method: http.MethodPost, path: "/api/users/{userID}/filters", id: "createFilter", tag: "filters",
			summary: "Create a filter; each user may have one filter of each type.", reqParams: filterUserPath{}, reqBody: apiv1.PostFilterReq{}, resp: apiv1.FilterResp{}, status: http.StatusCreated,
		},
		{
			method: http.MethodGet, path: "/api/users/{userID}/filters/{filterID}", id: "getFilter", tag: "filters",
			summary: "Get a filter.", reqParams: filterPath{}, resp: apiv1.FilterResp{}, status: http.StatusOK,
		},
		{
			method: http.MethodPut, path: "/api/users/{userID}/filters/{filterID}", id: "updateFilter", tag: "filters",
			summary: "Replace a filter's enabled state and configuration.", reqParams: filterPath{}, reqBody: apiv1.PutFilterReq{}, resp: apiv1.FilterResp{}, status: http.StatusOK,
		},
		{
			method: http.MethodDelete, path: "/api/users/{userID}/filters/{filterID}", id: "deleteFilter", tag: "filters",
			summary: "Delete a filter.", reqParams: filterPath{}, status: http.StatusNoContent,
		},
		{
			method: http.MethodGet, path: "/api/timeline", id: "getTimeline", tag: "timeline",
			summary:   "Get the paginated, curated timeline, optionally filtered by status and publish date.",
			reqParams: timelineQuery{}, resp: apiv1.TimelineResp{}, status: http.StatusOK,
		},
		{
			method: http.MethodGet, path: "/api/feed-entries/{feedEntryID}", id: "getFeedEntry", tag: "feed-entries",
			summary:   "Get the full, reader-mode content of a feed entry.",
			reqParams: feedEntryPath{}, resp: apiv1.FeedEntryResp{}, status: http.StatusOK,
		},
		{
			method: http.MethodGet, path: "/api/oauth-login/gh", id: "startGithubLogin", tag: "auth",
			summary:   "Start GitHub OAuth login; redirects the browser to GitHub.",
			reqParams: oauthLoginQuery{}, status: http.StatusFound,
		},
		{
			method: http.MethodGet, path: "/api/oauth-callback/gh", id: "githubOAuthCallback", tag: "auth",
			summary:   "GitHub OAuth callback: verifies state, ensures the user, sets the session cookie, and redirects to FRONTEND_URL.",
			reqParams: oauthCallbackQuery{}, status: http.StatusFound,
		},
		{
			method: http.MethodPost, path: "/api/logout", id: "logout", tag: "auth",
			summary: "Clear the session cookie.",
			status:  http.StatusNoContent,
		},
	}

	for _, o := range ops {
		oc, err := reflector.NewOperationContext(o.method, o.path)
		if err != nil {
			fail(fmt.Errorf("new operation context for %s %s: %w", o.method, o.path, err))
		}

		oc.SetID(o.id)
		oc.SetSummary(o.summary)
		oc.SetTags(o.tag)

		if o.reqBody != nil {
			oc.AddReqStructure(o.reqBody)
		}
		if o.reqParams != nil {
			oc.AddReqStructure(o.reqParams)
		}
		if o.resp != nil {
			oc.AddRespStructure(o.resp, openapi.WithHTTPStatus(o.status))
		} else {
			oc.AddRespStructure(nil, openapi.WithHTTPStatus(o.status))
		}

		if err := reflector.AddOperation(oc); err != nil {
			fail(fmt.Errorf("add operation for %s %s: %w", o.method, o.path, err))
		}
	}

	if err := applyFilterSchemaContract(reflector.SpecEns()); err != nil {
		fail(fmt.Errorf("apply filter schema contract: %w", err))
	}

	yamlBytes, err := reflector.Spec.MarshalYAML()
	if err != nil {
		fail(fmt.Errorf("marshal spec to yaml: %w", err))
	}

	if err := os.WriteFile(*out, yamlBytes, 0o644); err != nil {
		fail(fmt.Errorf("write %s: %w", *out, err))
	}
}

func applyFilterSchemaContract(spec *openapi3.Spec) error {
	// Round-trip through JSON maps to make targeted component-schema changes
	// while retaining every reflected field and leaving other schemas untouched.
	encoded, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal reflected spec: %w", err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return fmt.Errorf("decode reflected spec: %w", err)
	}
	components, ok := document["components"].(map[string]any)
	if !ok {
		return fmt.Errorf("components are missing")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		return fmt.Errorf("component schemas are missing")
	}

	post, err := schemaObject(schemas, "V1PostFilterReq")
	if err != nil {
		return err
	}
	put, err := schemaObject(schemas, "V1PutFilterReq")
	if err != nil {
		return err
	}
	response, err := schemaObject(schemas, "V1FilterResp")
	if err != nil {
		return err
	}
	listResponse, err := schemaObject(schemas, "V1FilterListResp")
	if err != nil {
		return err
	}

	postProps, err := schemaProperties(post)
	if err != nil {
		return err
	}
	putProps, err := schemaProperties(put)
	if err != nil {
		return err
	}
	responseProps, err := schemaProperties(response)
	if err != nil {
		return err
	}
	listResponseProps, err := schemaProperties(listResponse)
	if err != nil {
		return err
	}

	// A pointer bool detects omission in Go, while the API contract accepts only
	// an actual boolean value when the required property is present.
	if enabled, ok := postProps["enabled"].(map[string]any); ok {
		delete(enabled, "nullable")
	}
	if enabled, ok := putProps["enabled"].(map[string]any); ok {
		delete(enabled, "nullable")
	}
	post["oneOf"] = []any{
		filterVariant(postProps, "type", "ai", "ai_config", []string{"type", "enabled", "ai_config"}),
		filterVariant(postProps, "type", "list", "list_config", []string{"type", "enabled", "list_config"}),
	}
	delete(post, "properties")
	delete(post, "required")
	put["oneOf"] = []any{
		filterVariant(putProps, "", "", "ai_config", []string{"enabled", "ai_config"}),
		filterVariant(putProps, "", "", "list_config", []string{"enabled", "list_config"}),
	}
	delete(put, "properties")
	delete(put, "required")

	// Responses have non-null list arrays even though request decoding accepts
	// omitted or null list values, so only the response variant tightens them.
	listResponseVariant := filterVariant(responseProps, "type", "list", "list_config", []string{"id", "user_id", "type", "enabled", "list_config", "created_at", "updated_at"})
	responseVariantProps := listResponseVariant["properties"].(map[string]any)
	responseVariantProps["list_config"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"allowlist":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"disallowlist": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
	response["oneOf"] = []any{
		filterVariant(responseProps, "type", "ai", "ai_config", []string{"id", "user_id", "type", "enabled", "ai_config", "created_at", "updated_at"}),
		listResponseVariant,
	}
	delete(response, "properties")
	delete(response, "required")
	filtersArray, ok := listResponseProps["filters"].(map[string]any)
	if !ok {
		return fmt.Errorf("filter list response array schema is missing or malformed")
	}
	delete(filtersArray, "nullable")

	encoded, err = json.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal updated spec: %w", err)
	}
	if err := json.Unmarshal(encoded, spec); err != nil {
		return fmt.Errorf("decode updated spec: %w", err)
	}
	return nil
}

func schemaObject(schemas map[string]any, name string) (map[string]any, error) {
	schema, ok := schemas[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema %s is missing or malformed", name)
	}
	return schema, nil
}

func schemaProperties(schema map[string]any) (map[string]any, error) {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema properties are missing or malformed")
	}
	return properties, nil
}

func filterVariant(properties map[string]any, discriminator, value, config string, required []string) map[string]any {
	variant := make(map[string]any, len(properties))
	maps.Copy(variant, properties)
	for key := range variant {
		if key == "ai_config" && config != "ai_config" || key == "list_config" && config != "list_config" {
			delete(variant, key)
		}
	}
	if discriminator != "" {
		variant[discriminator] = map[string]any{"type": "string", "enum": []any{value}}
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": variant, "required": required,
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
