---
paths:
  - "internal/seymour/**"
---

# `internal/seymour` conventions

- This package defines the domain models and `Service` interfaces; DB types
  live here and are reused across the app.
- Timestamps are plain `time.Time` fields — the MySQL driver handles
  `DATETIME`/`TIMESTAMP` marshaling given `parseTime=true` on the DSN.
- Errors returned from any `Service` implementation should be a
  `*seymour.Error` (built via `seymour.E(...)`, or a sentinel like
  `seymour.ErrNotFound`/`seymour.ErrConflict`) rather than a plain `error`,
  so callers (`internal/api`, `internal/worker`) can use `errors.As` to
  recover the right HTTP status instead of falling back to a generic 500.
