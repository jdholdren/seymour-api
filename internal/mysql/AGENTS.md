# MySQL package conventions

- `sqlx` + `squirrel` query builder. Pure-Go driver (no CGO):
  `github.com/go-sql-driver/mysql`.
- IDs are UUIDs with a namespace suffix, e.g. `{uuid}-fd` for feeds.
- Connected via a DSN in the `DATABASE` env var, which must include
  `parseTime=true` (e.g.
  `user:pass@tcp(mysql:3306)/seymour?parseTime=true&multiStatements=true&clientFoundRows=true`)
  so the driver marshals `DATETIME`/`TIMESTAMP` natively into `time.Time`.
- Include `clientFoundRows=true` in the DSN so updates report matched rows,
  not changed rows. This lets `RowsAffected() == 0` mean "not found" even
  when a user saves identical values.
- Migrations are embedded Go files (`internal/migrations`), run via
  `golang-migrate`.
- Timeline entry statuses: `requires_judgement`, `approved`, `rejected`.
