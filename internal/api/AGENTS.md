# API package

`internal/api/auth.go`'s `requireAuth` middleware enforces the `session`
cookie on every route except `/api/viewer` (which doubles as the "who am I"
check), `/api/oauth-login/gh`, `/api/oauth-callback/gh`, and `/api/logout`.
`subscriptions` carry a `user_id`; `feeds`/`feed_entries`
stay a shared global cache (deduped by URL) with no owner.

## Endpoints

- `GET /api/viewer` — Viewer info; works logged-out too (returns empty
  subscriptions, no `user` field)
- `POST /api/users/{userID}/subscriptions` — Subscribe to feed (triggers
  creates/syncs the feed directly). `{userID}` must match the session's user
- `GET /api/users/{userID}/subscriptions` — List subscriptions for that
  user. `{userID}` must match the session's user
- `DELETE /api/subscriptions/{subscriptionID}` — Delete a subscription;
  ownership is checked by fetching the subscription and comparing its
  `user_id` to the session
- `GET /api/feed-entries/{feedEntryID}` — Full article content via
  go-readability; any authenticated user can read any entry (feeds/entries
  are a shared global cache, not user-owned)
- `GET /api/feeds/{feedID}/entries?page=1&size=20` — List a page of feed
  entries; requires a session, but any authenticated user can read any feed's
  entries. `page` is 1-based; `size` is 1–100 (default 20).
- `GET /api/oauth-login/gh` — Start GitHub OAuth login; redirects to GitHub.
  Accepts `?s=<path>` for where to send the browser (on `FRONTEND_URL`)
  after login succeeds, defaults to `/`
- `GET /api/oauth-callback/gh` — GitHub OAuth callback; verifies state,
  ensures the user via `UserService`, sets the `session` cookie, redirects
  to `FRONTEND_URL` + the requested path
- `POST /api/logout` — Clears the `session` cookie

## Env vars

`DATABASE` (MySQL DSN), `PORT` (default 4444), `CORS`,
`FRONTEND_URL` (browser is redirected here after GitHub OAuth completes),
`GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_REDIRECT_URL` (must
match the GitHub OAuth app's configured callback URL),
`SESSION_HASH_KEY`/`SESSION_BLOCK_KEY` (hex-encoded securecookie
signing/encryption keys)

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
