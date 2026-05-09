# Destructive actions

> Applies to: every session. Cross-cutting safety rule for actions that mutate shared or persistent state.

## Database writes require explicit per-turn confirmation

Never execute `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `DROP`, `ALTER`, or any other state-mutating SQL against any database — Supabase (local or hosted), Postgres, or otherwise — without **explicit user confirmation in the same turn**.

Prior authorization for related work does **not** carry over: "go ahead and run the migration" approves the migration named in that turn, not subsequent writes. Each new mutation needs its own go-ahead.

This applies to:

- `mcp__plugin_supabase_supabase__execute_sql` with anything other than `SELECT`.
- `mcp__plugin_supabase_supabase__apply_migration`.
- `psql` / `supabase db ...` / GORM auto-migrate run via `Bash`.
- Any test or seed script that writes to a non-throwaway database.

`SELECT` queries, `EXPLAIN`, schema introspection (`list_tables`, `\d`, etc.) are read-only and don't need confirmation.

When in doubt, show the user the exact SQL or command you're about to run and wait for `yes` / `go ahead` before executing.

## Other destructive actions

The same per-turn-confirmation rule applies to operations the system prompt already calls out — `git reset --hard`, `git push --force`, branch deletion, `rm -rf`, killing user processes, modifying CI/CD pipelines, sending external messages (Slack, email, GitHub comments). One approval is for one action; ask again for the next.
