# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Seymour?

Seymour is an RSS feed aggregator with a curated timeline, moving from single-tenant to multi-tenant. Users subscribe to RSS feeds, and a Temporal worker syncs feeds, builds a timeline, then judges entries to decide what gets surfaced. The frontend is a separate project (expected at localhost:3000).

A `UserService` (`internal/seymour/user.go`, implemented in `internal/sqlite/users.go`) and GitHub OAuth login (`internal/api/oauth.go`) exist. `internal/api/auth.go`'s `requireAuth` middleware enforces the session cookie on every route except `/api/viewer` (which doubles as the "who am I" check), `/api/oauth-login/gh`, `/api/oauth-callback/gh`, and `/api/logout`. `subscriptions`/`timeline_entries` carry a `user_id`; `feeds`/`feed_entries` stay a shared global cache (deduped by URL) with no owner.

The judging step is a seam: `activities.JudgeEntries` in `internal/worker/judge.go` currently approves every entry. Replace its body to introduce a real curation strategy — the surrounding workflow, batching, and persistence already exist.

## Common Commands

- `make test` — Run all tests (`go test ./...`)
- `go test -run TestName ./internal/path/...` — Run a single test
- `make up-d` — Start all services via docker compose, detached (use this one from an agent session — `make up`/`docker compose up` blocks in the foreground and will hang the session)
- `make build` — Build all Docker images
- `make rb-api` — Rebuild and restart only the API service
- `make rb-worker` — Rebuild and restart only the worker service

## Architecture

Two binaries, both in `cmd/`:

- **`cmd/api`** — REST API server (port 4444). HTTP handlers in `internal/api/`. Uses Gorilla Mux for routing.
- **`cmd/worker`** — Temporal workflow worker. Workflows and activities in `internal/worker/`.

### Core packages

- **`internal/seymour`** — Domain models and the `Service` interfaces. DB types are defined and reused here across the app. `DBTime` is a custom type for SQLite datetime marshaling using RFC3339. Errors returned from any `Service` implementation should be a `*seymour.Error` (built via `seymour.E(...)`, or one of the sentinels like `seymour.ErrNotFound`/`seymour.ErrConflict`) whenever possible, rather than a plain `error`, so callers (`internal/api`, `internal/worker`) can rely on `errors.As` to recover the right HTTP status instead of falling back to a generic 500.
- **`internal/sqlite`** — SQLite implementation of `Service`'s. Uses `sqlx` + `squirrel` query builder. Pure-Go SQLite driver (no CGO): `modernc.org/sqlite`.
- **`internal/sync`** — RSS feed parsing and sync logic. Parses XML, sanitizes HTML, extracts feed metadata.
- **`internal/worker`** — Temporal workflows and activities:
  - `SyncAllFeeds` — Scheduled every 15 min, batches feeds in groups of 50
  - `CreateFeed` — Creates feed, syncs, rolls back on failure
  - `RefreshTimeline` — Inserts missing timeline entries, triggers judging
  - `JudgeTimeline` — Approves/rejects entries via `JudgeEntries` (batches of `judgeBatchSize`, max 3 loops)
- **`internal/migrations`** — Embedded SQL migration files, run via `golang-migrate`

### Temporal patterns

- Task queue name: `shared`
- Singleton workflows use `WorkflowIDReusePolicy: TERMINATE_IF_RUNNING`
- Schedules: `sync_all` and `refresh_timelines` both run every 15 minutes
- Child workflows use `ParentClosePolicy: ABANDON` so they outlive parents

### ID generation

UUIDs with namespace suffixes: e.g. `{uuid}-fd` for feeds. See the `internal/sqlite` package.

### Database

SQLite with connection flags `-txlock=immediate -busy_timeout=5000`. Migrations are embedded Go files. Timeline entry statuses: `requires_judgement`, `approved`, `rejected`.

## Environment Variables

**API:** `DATABASE` (SQLite path), `TEMPORAL_HOST_PORT`, `PORT` (default 4444), `CORS`, `FRONTEND_URL` (browser is redirected here after GitHub OAuth completes), `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_REDIRECT_URL` (must match the GitHub OAuth app's configured callback URL), `SESSION_HASH_KEY`/`SESSION_BLOCK_KEY` (hex-encoded securecookie signing/encryption keys)
**Worker:** `DATABASE`, `TEMPORAL_HOST_PORT`

## API Endpoints

All routes below except `/api/viewer`, `/api/oauth-login/gh`, `/api/oauth-callback/gh`, and `/api/logout` require a valid `session` cookie (enforced by `requireAuth` middleware in `internal/api/auth.go`).

- `GET /api/viewer` — Viewer info; works logged-out too (returns empty subscriptions, no `user` field)
- `POST /api/users/{userID}/subscriptions` — Subscribe to feed (triggers CreateFeed workflow). `{userID}` must match the session's user
- `GET /api/users/{userID}/subscriptions` — List subscriptions for that user. `{userID}` must match the session's user
- `DELETE /api/subscriptions/{subscriptionID}` — Delete a subscription; ownership is checked by fetching the subscription and comparing its `user_id` to the session
- `GET /api/users/{userID}/timeline` — Paginated curated timeline for that user (supports `feed_id`, `status` — one of `requires_judgement`/`approved`/`rejected`, defaults to all — and `from`/`to` publish-date filters, RFC3339 or `YYYY-MM-DD`). `{userID}` must match the session's user
- `GET /api/feed-entries/{feedEntryID}` — Full article content via go-readability; any authenticated user can read any entry (feeds/entries are a shared global cache, not user-owned)
- `GET /api/oauth-login/gh` — Start GitHub OAuth login; redirects to GitHub. Accepts `?s=<path>` for where to send the browser (on `FRONTEND_URL`) after login succeeds, defaults to `/`
- `GET /api/oauth-callback/gh` — GitHub OAuth callback; verifies state, ensures the user via `UserService`, sets the `session` cookie, redirects to `FRONTEND_URL` + the requested path
- `POST /api/logout` — Clears the `session` cookie

## Core Dev Loop

The dev loop consists of two main loops, one for making quick edits and changes, the other for testing changes end-to-end:
1. Building Loop
2. After-Building Loop

### Building Loop

1. Make edits
2. Use gopls to detect breakages in the build
3. Write tests (work with user if needing input on what are good tests to have)
4. Run tests with `go test` targeting the affected packages (or ./... if the package is common enough)
5. Repeat until satisfied, then go to after-building loop

### After-Building Loop

1. Make sure the server is started via docker compose -d
2. If not, ask the user to start docker desktop
3. After making changes, use docker compose to rebuild and restart the changed container (api for api changes, worker for any temporal-related changes, restart both for common changes)
4. Changes are now live
5. Repeat until behavior is verified

# The gopls MCP server

These instructions describe how to efficiently work in the Go programming language using the gopls MCP server. You can load this file directly into a session where the gopls MCP server is connected.

## Detecting a Go workspace

At the start of every session, you MUST use the `go_workspace` tool to learn about the Go workspace. ONLY if you are in a Go workspace, you MUST run `go_vulncheck` immediately afterwards to identify any existing security risks. The rest of these instructions apply whenever that tool indicates that the user is in a Go workspace.

## Go programming workflows

These guidelines MUST be followed whenever working in a Go workspace. There are two workflows described below: the 'Read Workflow' must be followed when the user asks a question about a Go workspace. The 'Edit Workflow' must be followed when the user edits a Go workspace.

You may re-do parts of each workflow as necessary to recover from errors. However, you must not skip any steps.

### Read workflow

The goal of the read workflow is to understand the codebase.

1. **Understand the workspace layout**: Start by using `go_workspace` to understand the overall structure of the workspace, such as whether it's a module, a workspace, or a GOPATH project.

2. **Find relevant symbols**: If you're looking for a specific type, function, or variable, use `go_search`. This is a fuzzy search that will help you locate symbols even if you don't know the exact name or location.
   EXAMPLE: search for the 'Server' type: `go_search({"query":"server"})`

3. **Understand a file and its intra-package dependencies**: When you have a file path and want to understand its contents and how it connects to other files *in the same package*, use `go_file_context`. This tool will show you a summary of the declarations from other files in the same package that are used by the current file. `go_file_context` MUST be used immediately after reading any Go file for the first time, and MAY be re-used if dependencies have changed.
   EXAMPLE: to understand `server.go`'s dependencies on other files in its package: `go_file_context({"file":"/path/to/server.go"})`

4. **Understand a package's public API**: When you need to understand what a package provides to external code (i.e., its public API), use `go_package_api`. This is especially useful for understanding third-party dependencies or other packages in the same monorepo.
   EXAMPLE: to see the API of the `storage` package: `go_package_api({"packagePaths":["example.com/internal/storage"]})`

### Editing workflow

The editing workflow is iterative. You should cycle through these steps until the task is complete.

1. **Read first**: Before making any edits, follow the Read Workflow to understand the user's request and the relevant code.

2. **Find references**: Before modifying the definition of any symbol, use the `go_symbol_references` tool to find all references to that identifier. This is critical for understanding the impact of your change. Read the files containing references to evaluate if any further edits are required.
   EXAMPLE: `go_symbol_references({"file":"/path/to/server.go","symbol":"Server.Run"})`

3. **Make edits**: Make the required edits, including edits to references you identified in the previous step. Don't proceed to the next step until all planned edits are complete.

4. **Check for errors**: After every code modification, you MUST call the `go_diagnostics` tool. Pass the paths of the files you have edited. This tool will report any build or analysis errors.
   EXAMPLE: `go_diagnostics({"files":["/path/to/server.go"]})`

5. **Fix errors**: If `go_diagnostics` reports any errors, fix them. The tool may provide suggested quick fixes in the form of diffs. You should review these diffs and apply them if they are correct. Once you've applied a fix, re-run `go_diagnostics` to confirm that the issue is resolved. It is OK to ignore 'hint' or 'info' diagnostics if they are not relevant to the current task. Note that Go diagnostic messages may contain a summary of the source code, which may not match its exact text.

6. **Check for vulnerabilities**: If your edits involved adding or updating dependencies in the go.mod file, you MUST run a vulnerability check on the entire workspace. This ensures that the new dependencies do not introduce any security risks. This step should be performed after all build errors are resolved. EXAMPLE: `go_vulncheck({"pattern":"./..."})`

7. **Run tests**: Once `go_diagnostics` reports no errors (and ONLY once there are no errors), run the tests for the packages you have changed. You can do this with `go test [packagePath...]`. Don't run `go test ./...` unless the user explicitly requests it, as doing so may slow down the iteration loop.
