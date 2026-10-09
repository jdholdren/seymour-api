# `internal/seymour` conventions

- This package defines the domain models and `Service` interfaces; DB types
  live here and are reused across the app.
- Timestamps are plain `time.Time` fields — the MySQL driver handles
  `DATETIME`/`TIMESTAMP` marshaling given `parseTime=true` on the DSN.
- Partial user updates use `*sql.NullString` fields: a nil pointer leaves the
  field unchanged, `Valid: false` clears it to SQL `NULL`, and `Valid: true`
  stores the supplied string (including an empty string).
- Errors returned from any `Service` implementation should be a
  `*seymour.Error` (built via `seymour.E(...)`, or a sentinel like
  `seymour.ErrNotFound`/`seymour.ErrConflict`) rather than a plain `error`,
  so callers (`internal/api`, `internal/worker`) can use `errors.As` to
  recover the right HTTP status instead of falling back to a generic 500.
- Generate interface mocks in `internal/mock` with `go generate ./internal/seymour`.
  Use the registered `mockgen` Go tool; when adding an interface, include it in
  the generation directive in `seymour.go`.
