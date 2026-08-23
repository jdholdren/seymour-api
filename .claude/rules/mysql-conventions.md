---
paths:
  - "internal/mysql/**"
  - "internal/migrations/**"
---

# MySQL package conventions

- `sqlx` + `squirrel` query builder. Pure-Go driver (no CGO):
  `github.com/go-sql-driver/mysql`.
- IDs are UUIDs with a namespace suffix, e.g. `{uuid}-fd` for feeds.
- Connected via a DSN in the `DATABASE` env var, which must include
  `parseTime=true` (e.g.
  `user:pass@tcp(mysql:3306)/seymour?parseTime=true&multiStatements=true`)
  so the driver marshals `DATETIME`/`TIMESTAMP` natively into `time.Time`.
- Migrations are embedded Go files (`internal/migrations`), run via
  `golang-migrate`.
- Timeline entry statuses: `requires_judgement`, `approved`, `rejected`.
