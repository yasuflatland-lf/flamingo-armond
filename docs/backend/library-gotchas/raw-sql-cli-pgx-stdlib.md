# Raw SQL CLI: `sql.Open("pgx", dbURL)` + pgx stdlib, not GORM

> Part of the [Go library gotchas](./../../../.claude/rules/go-library-gotchas.md) rules.

## Pattern

Single-purpose CLI tools connect to Postgres using `database/sql` with the pgx stdlib driver, not GORM.

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

db, err := sql.Open("pgx", dbURL)
if err != nil {
    return eris.Wrap(err, "open db")
}
defer db.Close()

if err := db.Ping(); err != nil {
    return eris.Wrap(err, "ping db")
}
```

## Why not GORM

GORM is the right tool for the application server, where model-layer features (auto-migrations, associations, soft-delete, hooks) earn their overhead. For a one-shot CLI tool:

- GORM's connection pool and model registry are initialized even if only one query runs.
- GORM wraps raw SQL results in reflection-heavy scanning that adds no value when the CLI is already writing its own query strings.
- A plain `*sql.DB` lifecycle is straightforward to reason about: open, ping, query, close. No `gorm.DB` state leaks between calls.

## Validate the DSN before calling `sql.Open`

`sql.Open("pgx", "")` succeeds — the driver defers connection establishment. The first observable failure is `db.Ping()`, which returns a pgx-level error such as:

```
failed to connect to `host=localhost user=postgres database=`: dial error
```

This error message does not make it obvious that the DSN was never provided. Always validate the flag value before calling `sql.Open`:

```go
if dbURL == "" {
    return eris.New("--db-url is required")
}
db, err := sql.Open("pgx", dbURL)
```

## `auth.users` requires the superuser DSN

The Supabase `auth` schema is owned by the `supabase_auth_admin` role and is not readable by the normal application role (e.g. `anon`, `authenticated`, or a custom app role). A CLI that queries `auth.users` directly — for example to look up emails from UUIDs — must connect with the postgres/superuser DSN, not the application DSN.

Concretely, if the tool accepts both a `--db-url` (application DSN) and a `--admin-db-url` (superuser DSN), the `auth.users` query must use the admin connection:

```go
adminDB, err := sql.Open("pgx", adminDBURL)
// ...
rows, err := adminDB.Query(`SELECT id, email FROM auth.users WHERE id = ANY($1)`, pq.Array(ids))
```

Attempting the same query over the application DSN returns `ERROR: permission denied for schema auth` — not a confusing query result, but the error is only visible at runtime, so the DSN choice must be explicit in the code.
