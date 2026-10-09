# API package

`internal/api/auth.go`'s `requireAuth` middleware enforces the `session`
cookie on every route except `/api/viewer` (which doubles as the "who am I"
check), `/api/oauth-login/gh`, `/api/oauth-callback/gh`, and `/api/logout`.
`subscriptions`/`timeline_entries` carry a `user_id`; `feeds`/`feed_entries`
stay a shared global cache (deduped by URL) with no owner.

`apis/v1/date.go`'s `Date` represents a calendar day (no time-of-day), for
API fields/params that are date blocks rather than instants (e.g. the
timeline's `from`/`to` filters). Its zero value means "unset" (`IsZero()`),
so prefer a plain `Date` over `*Date` in structs — don't reach for a pointer
just to express absence.

## Endpoints

- `GET /api/viewer` — Viewer info; works logged-out too (returns empty
  subscriptions, no `user` field)
- `POST /api/users/{userID}/subscriptions` — Subscribe to feed (triggers
  `CreateFeed` workflow). `{userID}` must match the session's user
- `GET /api/users/{userID}/subscriptions` — List subscriptions for that
  user. `{userID}` must match the session's user
- `DELETE /api/subscriptions/{subscriptionID}` — Delete a subscription;
  ownership is checked by fetching the subscription and comparing its
  `user_id` to the session
- `GET /api/users/{userID}/timeline` — Paginated curated timeline for that
  user (supports `feed_id`, `status` — one of
  `requires_judgement`/`approved`/`rejected`, defaults to all — and
  `from`/`to` publish-date filters as `YYYY-MM-DD`, parsed via
  `apiv1.ParseDate`). `{userID}` must match the session's user
- `GET /api/users/{userID}/timeline-prompt` — Get the user's timeline prompt
  (unset is returned as an empty string); `{userID}` must match the session
- `PUT /api/users/{userID}/timeline-prompt` — Set the user's timeline prompt;
  `prompt` is limited to 1000 Unicode characters. An empty, omitted, or
  null prompt clears it. `{userID}` must match the session
- `GET /api/feed-entries/{feedEntryID}` — Full article content via
  go-readability; any authenticated user can read any entry (feeds/entries
  are a shared global cache, not user-owned)
- `GET /api/oauth-login/gh` — Start GitHub OAuth login; redirects to GitHub.
  Accepts `?s=<path>` for where to send the browser (on `FRONTEND_URL`)
  after login succeeds, defaults to `/`
- `GET /api/oauth-callback/gh` — GitHub OAuth callback; verifies state,
  ensures the user via `UserService`, sets the `session` cookie, redirects
  to `FRONTEND_URL` + the requested path
- `POST /api/logout` — Clears the `session` cookie

## Env vars

`DATABASE` (MySQL DSN), `TEMPORAL_HOST_PORT`, `PORT` (default 4444), `CORS`,
`FRONTEND_URL` (browser is redirected here after GitHub OAuth completes),
`GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_REDIRECT_URL` (must
match the GitHub OAuth app's configured callback URL),
`SESSION_HASH_KEY`/`SESSION_BLOCK_KEY` (hex-encoded securecookie
signing/encryption keys)

## Handler tests

Test handlers directly with `httptest` requests and response recorders rather
than constructing the full server/router. Set the authenticated user ID in the
request context (`userIDCtxKey`) and path parameters with `mux.SetURLVars`.
Check returned handler errors directly; cookie decoding and authentication
middleware belong in their own tests, not handler tests.

Use Uber gomock with the generated services in `internal/mock`, rather than
hand-written service mocks. Keep cases focused on a single handler call.
Centralize server and mock setup in `newRig(t)`, returning a rig that exposes
`server` and every mock it creates (`users`, `feeds`, and `timeline`). The helper
calls `t.Helper()`, creates a gomock controller, and wires the mocks into the
server. Reuse the rig in each test: configure expectations on `rig.users` (or
another exposed mock), then call the handler directly on `rig.server`. Keep
request construction and authentication context explicit in the test.

Prefer standalone tests named for the handler and expected behavior (for example,
`TestGetTimelinePromptReturnsStoredPrompt`), rather than broad feature tests or
subtests grouping distinct behaviors.
Test application behavior (validation limits, service arguments, responses, and
ownership), not JSON decoding or other behavior already provided by libraries.

## Response shape checks

Handler functions in this package don't declare a return type — the response
shape is whatever gets passed to `writeJSON(w, status, ...)` inside the
handler body. Because of that, the compiler won't catch a breaking change to
a response shape the way it would for a typed return value.

Whenever you touch a handler in this package (or a helper it calls to build
its response, e.g. `apiSubscription`), read the full body of
that handler down to its `writeJSON` call(s) and check whether the shape
being written has changed in a breaking way for existing clients:
- a field removed or renamed
- a field's type changed (e.g. `string` -> `*string`, flattened -> nested)
- the top-level struct swapped for a different one (e.g. returning
  `SubscriptionResp` where `FeedResp` used to go out)
- a previously-required field now omitted/empty in some path

If you find a breaking change, call it out explicitly to the user rather than
assuming it's fine — even if the new shape is arguably better. Additive
changes (new optional fields) aren't breaking and don't need a callout.
