// Command genopenapi generates the OpenAPI 3.0 spec for the Seymour API from
// the request/response types already defined in internal/api, using
// reflection (github.com/swaggest/openapi-go). It's meant to be regenerated
// whenever the API's wire types or routes change (see `make gen-openapi`),
// and CI checks that the committed spec matches what this program produces.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/jdholdren/seymour/internal/api"
)

// Doc-only structs describing query/path parameters. These mirror what the
// handlers in internal/api actually read off the request (see
// parsePaginationParams, mux.Vars, r.URL.Query()) but live here instead of in
// internal/api so the generator doesn't need to change handler behavior.
type (
	timelineQuery struct {
		FeedID string `query:"feed_id" description:"Filter the timeline to a single feed."`
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
		reqBody, reqParams, resp       interface{}
		status                         int
	}

	ops := []op{
		{
			method: http.MethodGet, path: "/api/viewer", id: "getViewer", tag: "viewer",
			summary: "Get info about the current viewer, including their subscriptions.",
			resp:    api.Viewer{}, status: http.StatusOK,
		},
		{
			method: http.MethodPost, path: "/api/subscriptions", id: "createSubscription", tag: "subscriptions",
			summary: "Subscribe to a feed by URL, triggering the CreateFeed workflow.",
			reqBody: api.PostSubscriptionReq{}, resp: api.FeedResp{}, status: http.StatusCreated,
		},
		{
			method: http.MethodGet, path: "/api/subscriptions", id: "listSubscriptions", tag: "subscriptions",
			summary: "List the viewer's subscriptions.",
			resp:    api.SubscriptionListResp{}, status: http.StatusCreated,
		},
		{
			method: http.MethodGet, path: "/api/timeline", id: "getTimeline", tag: "timeline",
			summary:   "Get the paginated, curated timeline of approved entries.",
			reqParams: timelineQuery{}, resp: api.TimelineResp{}, status: http.StatusOK,
		},
		{
			method: http.MethodGet, path: "/api/feed-entries/{feedEntryID}", id: "getFeedEntry", tag: "feed-entries",
			summary:   "Get the full, reader-mode content of a feed entry.",
			reqParams: feedEntryPath{}, resp: api.FeedEntryResp{}, status: http.StatusOK,
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

	yamlBytes, err := reflector.Spec.MarshalYAML()
	if err != nil {
		fail(fmt.Errorf("marshal spec to yaml: %w", err))
	}

	if err := os.WriteFile(*out, yamlBytes, 0o644); err != nil {
		fail(fmt.Errorf("write %s: %w", *out, err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
