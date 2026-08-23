---
paths:
  - "internal/api/**"
  - "apis/v1/**"
---

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
