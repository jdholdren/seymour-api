---
type: workflow
title: Authentication and account lifecycle
description: Traces GitHub OAuth state verification, account resolution, signed session cookies, and the middleware that protects user-scoped API routes.
tags: [authentication, oauth, sessions, authorization]
verified:
  - by: openwiki/0.9.0
    at: 2026-11-27T18:34:43.213Z
sources:
  - id: openwiki-source-c0c2d534b0f007c7a8cdcafd
    resource: repo://internal/api/AGENTS.md
  - id: openwiki-source-e25cfa6b3c3cb5e2d305609f
    resource: repo://internal/api/api.go
  - id: openwiki-source-57d31608bc370e75d59e9118
    resource: repo://internal/api/oauth.go
  - id: openwiki-source-e4e1ab9bb2b6b8f22574b318
    resource: repo://internal/api/timeline.go
  - id: openwiki-source-4a9458da95aa2d84436998c0
    resource: repo://internal/api/viewer.go
generated: { by: "opencode", at: "2026-11-27T18:34:43.213Z" }
---

# Authentication and account lifecycle

## Public and protected routes

`NewServer` registers viewer, GitHub OAuth start/callback, and logout as public routes. Other `/api` routes are attached to a subrouter guarded by `requireAuth`. The middleware decodes the `session` cookie and attaches its user ID to request context; handlers use that identity for ownership checks (`internal/api/api.go`, `internal/api/auth.go`).

## GitHub OAuth round trip

The login endpoint generates a random state value and stores it, along with a post-login redirect path, in a signed/encrypted `oauth_state` cookie. The requested `s` path is accepted only when it begins with one slash and not two; otherwise the destination defaults to `/`. The state cookie is HttpOnly, SameSite=Lax, TLS-secure when the request uses TLS, and expires after ten minutes (`internal/api/oauth.go`).

On callback, the server requires and decodes the state cookie, compares its state with the callback query, and rejects missing, invalid, or mismatched state. It clears the temporary cookie, exchanges the authorization code, fetches the GitHub user ID, and calls `UserService.EnsureUser` with the GitHub identity. A successful user resolution produces a signed/encrypted `session` cookie valid for 30 days, then redirects to the configured frontend URL and the validated path (`internal/api/oauth.go`, `internal/mysql/users.go`).

The identity-provider identifier is stored separately from the Seymour user ID. `EnsureUser` returns the existing mapping when present, or creates user and login records transactionally; concurrent creation resolves through the unique provider identity constraint (`internal/seymour/user.go`, `internal/mysql/users.go`).

## Session and authorization semantics

`sessionUserID` treats a missing or undecodable session cookie as logged out. Protected requests without a valid session receive a JSON unauthorized response. The viewer endpoint is intentionally public: logged-out callers receive an empty subscription map, while authenticated callers receive user details and subscriptions assembled from the user's subscription records and shared feeds (`internal/api/auth.go`, `internal/api/viewer.go`).

Authentication establishes identity but handlers still enforce resource ownership. Subscription list/create routes require the path user ID to match the session user; delete checks the subscription owner. Feed entries remain globally shared, so any authenticated user can read an entry (`internal/api/timeline.go`, `internal/api/AGENTS.md`).

Logout expires the session cookie and returns no content. Both state and session cookies use the request's TLS status to set the Secure attribute; session signing/encryption keys are decoded from required hex configuration at startup (`internal/api/oauth.go`, `cmd/api/main.go`).

## Related topics

- [Service architecture](../architecture/service-overview.md) maps route and service wiring.
- [Development and deployment](../operations/development-and-deployment.md) lists required runtime settings.
- [Subscriptions and reader requests](subscriptions-and-reading.md) details ownership behavior.
