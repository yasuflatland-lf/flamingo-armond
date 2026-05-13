# Backend database

> Pool / interface design, migrations (golang-migrate), `schema_migrations` and RLS, `SECURITY DEFINER` helper recipe, startup ordering, env vars. See `docs/backend.md` for runtime, `docs/playbook-patterns.md` § "Recovering from a dirty migration" for incident recovery, and `.claude/rules/go-library-gotchas.md` for GORM-specific quirks.

## Database

### Pool and interface design

A single `pgxpool.Pool` is created at startup. `stdlib.OpenDBFromPool` converts it into a `*sql.DB`, which is then handed to GORM. The result is **one pool, two interfaces** — pgx native for low-level queries and GORM for the ORM layer — without double-consuming Render free tier's connection limit.

### Migrations

Migrations use `golang-migrate` with `*.up.sql` / `*.down.sql` files. Raw SQL lets you express triggers, foreign keys, and `SECURITY DEFINER` functions directly, none of which GORM's AutoMigrate can model. AutoMigrate is therefore not used.

Migration files live under `backend/internal/database/migrations/`. Go's `//go:embed` directive does not allow `..` path components, so the migration directory must sit inside the package tree rather than at the repo root.

**Filename format: `yyyymmddhhmmss_<short_snake_case_description>.{up,down}.sql`** — the 14-digit timestamp prefix is the numeric version `golang-migrate` records in `public.schema_migrations` and uses to order files. New migrations therefore need a timestamp strictly greater than every existing file (UTC is fine; the values just need to sort correctly). The trailing description is for human readers and is not parsed — pick a short verb-led summary like `create_cards`, `enable_rls_deny_all`, or `initial_schema`. Up and down halves must share the same prefix and description so `golang-migrate` can pair them.

When a migration fails mid-run, `schema_migrations.dirty=true` is set. Recovery requires an operator to run `migrate force <version>`. The `run()` function treats any migration error as fatal and returns immediately (fail-fast). See `docs/playbook-patterns.md` § "Recovering from a dirty migration" for the operator runbook.

**`UPDATE schema_migrations SET dirty = false` alone is not enough.** A failed up migration leaves both `dirty = true` AND `version` pointing at the failed migration. Clearing only the dirty flag leaves the version pointing at the failed file, so the next `Steps(1)` call goes looking for migration `<failed+1>` and fails with `os.ErrNotExist`. The idiomatic recovery is `m.Force(<predecessor_version>)` — it rewrites both fields atomically and lets `Steps(1)` re-apply the original migration. The same gotcha applies to test code that simulates a dirty state via the migrate Go API; see `MigrateForceForTest` in `internal/database/export_test.go`.

#### golang-migrate transaction behaviour

**The `pgx/v5` driver does NOT auto-wrap each migration file in a transaction.** Every migration that requires atomicity must open its own `BEGIN; ... COMMIT;` block explicitly. A migration file that omits `BEGIN/COMMIT` and mixes DDL with privilege-sensitive statements (e.g. `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`) can partially succeed: the DDL commits, the later statement fails, and `schema_migrations.dirty=true` persists because golang-migrate commits that flag in its own separate transaction *before* it begins executing the migration SQL.

The structural consequence: **sensitive ALTER operations (RLS, GRANT, REVOKE) belong in their own dated migration file**, separate from the DDL that creates the tables. A failure in one file leaves the other file's work untouched, limiting blast radius.

#### Migration test quality bar

Tests that invoke the migration runner must assert *post-conditions*, not just "migrate ran without error". The model is `TestMigrations_AllPublicTablesHaveRLSEnabled` in `internal/database/pool_test.go`: it queries `pg_class` after migration and asserts every expected table has `relrowsecurity = true`. RLS policy behavior is covered by `internal/database/rls_test.go`, which connects as the Supabase-style `authenticated` role and sets local JWT claims before querying. Procedural success alone does not verify security posture.

**Every migration that adds an RLS policy must add a corresponding sub-test to `internal/database/rls_test.go`.** The sub-test must cover three cases: the owning user can read their own rows, a different user cannot read those rows, and the admin user can read any user's rows. For tables that restrict writes (INSERT/UPDATE), add cases that confirm permitted writes succeed and cross-user writes are denied. The `swipe_records` and `user_card_fsrs` sub-tests in `TestRLSPolicies_AuthenticatedRole` are the canonical models. A migration that enables RLS without a companion test leaves the policy behavior unverified at the code level.

#### `schema_migrations` and RLS

`public.schema_migrations` is golang-migrate's internal bookkeeping table. It is intentionally **excluded from the RLS-enable migration** for two reasons: (a) golang-migrate connects as the table owner, which in PostgreSQL bypasses RLS unless `FORCE ROW LEVEL SECURITY` is set, so enabling RLS on `schema_migrations` adds no security value; (b) if ownership ever changes and RLS without policies takes effect, golang-migrate would be blocked from updating the version record, bricking future deploys. Leave `schema_migrations` without RLS.

**Renaming or renumbering migration files is not transparent to the DB.** `golang-migrate` records the numeric version of each applied migration in `public.schema_migrations`. Renaming a file (e.g. `0001_create_profiles.up.sql` → `20250101000000_create_profiles.up.sql`) rewrites the source tree but **not** the DB row, so the next boot fails with `no migration found for version <N>: read down for version <N> migrations: file does not exist` — migrate's source-state reconciliation expects the recorded version to exist on disk. When the rename is identifier-only (up/down SQL bodies are byte-identical, `git log -M` reports an `R100` rename), the safe recovery is `UPDATE public.schema_migrations SET version = <new_version>, dirty = false WHERE version = <old_version>` against the production DB; the runbook lives in `playbooks/setup-prod/recover-migration-version-rebase.sql`. Do **not** apply this shortcut when the rename also changed migration content — in that case, squash the changes and use `migrate force <version>` against a known-good source state so the new content actually runs.

#### SECURITY DEFINER helper recipe

`SECURITY DEFINER` SQL functions used by RLS policies (e.g. `public.is_admin(uid uuid)`) must replicate this exact shape — each attribute has a load-bearing reason:

```sql
CREATE OR REPLACE FUNCTION public.is_admin(uid uuid)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$ ... $$;

REVOKE ALL ON FUNCTION public.is_admin(uuid) FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT EXECUTE ON FUNCTION public.is_admin(uuid) TO authenticated;
    END IF;
END
$$;
```

- `STABLE`, **not** `IMMUTABLE` — the function reads tables, and marking a table-reading function `IMMUTABLE` corrupts the planner's plan cache (the planner assumes the result is constant for fixed inputs).
- `SECURITY DEFINER` — the function executes as its owner, so RLS-enabled callers can probe role membership without needing direct read on `roles` / `user_roles`.
- `SET search_path = public` — neutralises the classic `SECURITY DEFINER` injection vector where an attacker creates a `pg_temp` shim function (e.g. their own `roles` table) that the function would otherwise resolve before the real one.
- `REVOKE ALL FROM PUBLIC` then narrow `GRANT EXECUTE` — without revoking from `PUBLIC`, anonymous PostgREST callers (`anon` role) could invoke the helper as an oracle to enumerate role assignments. The grant is intentionally limited to the Supabase-managed `authenticated` role.
- `DO $$ ... IF EXISTS pg_roles ... GRANT END $$` portability guard — plain PostgreSQL does not include Supabase-managed roles (`authenticated`, `anon`, `service_role`) by default. Wrapping role-specific GRANTs in this conditional `DO` block keeps the migration applicable outside Supabase. Testcontainers create a minimal `authenticated` role fixture so RLS behavior can be exercised directly. The Go backend connects as the table owner and bypasses RLS, so the GRANT path is used only by direct PostgREST / Edge callers in production.

### Postgres upsert: prerequisite UNIQUE / EXCLUSION constraint

`INSERT ... ON CONFLICT (cols) ...` requires the column set to be backed by a UNIQUE constraint, UNIQUE INDEX, or EXCLUSION constraint. Plain (non-unique) indexes and CHECK constraints do not satisfy the requirement — Postgres rejects the statement with SQLSTATE `42P10` "there is no unique or exclusion constraint matching the ON CONFLICT specification". This is checked at planning time, so the failure surfaces immediately, not on a colliding row.

The repository's two upsert call sites are deliberately asymmetric. `cards (cardgroup_id, front)` is backed by `uq_cards_cardgroup_front` (added in `20260503000000_add_cards_upsert_index`) because the dictionary upsert pipeline requires the constraint to dedup imports. `cardgroups (owner_id, name)` is **not** backed by any UNIQUE — the product allows the same user to create two cardgroups with the same name, and the GraphQL mutation validates only the name's length. A caller (production code or test helper) that issues `ON CONFLICT (owner_id, name)` against `cardgroups` is therefore invalid by schema design, not by a missing constraint that ought to be added.

### Postgres upsert: classifying inserted vs updated rows in one round-trip

`INSERT ... ON CONFLICT (key) DO UPDATE ... RETURNING (xmax = 0) AS inserted` lets a single statement report which rows were inserted and which were updated. PostgreSQL marks freshly inserted rows with `xmax = 0` and ON-CONFLICT-updated rows with `xmax = current_xid`, so the boolean expression in `RETURNING` classifies each row inline — no second `SELECT`, no application-side bookkeeping. The `cards` upsert path uses this in `cardRepo.UpsertManyTx` to compute the `inserted` / `updated` split.

The flip side is that the conflict key MUST be unique within the input batch. If two rows in the same `INSERT ... VALUES (...), (...)` collide on the conflict target, Postgres raises SQLSTATE `21000` ("ON CONFLICT DO UPDATE command cannot affect row a second time") and aborts the whole statement — Postgres deliberately does not silently merge intra-batch duplicates because either-row-wins is non-deterministic. The application layer must dedup by conflict key before issuing the SQL; the dictionary usecase keeps the *last* occurrence and reports earlier ones as soft errors.

### `users.last_viewed_cardgroup_id` — nullable FK with `ON DELETE SET NULL`

`users` carries a nullable `last_viewed_cardgroup_id uuid REFERENCES cardgroups(id) ON DELETE SET NULL` column to remember the cardgroup a returning user most recently studied. Three design choices are load-bearing:

- **Nullable + default null** — every existing user row stays valid without a backfill; the migration is forward-only and the down half drops the column cleanly. A NOT NULL with a default would force an arbitrary cardgroup choice for users who have never visited `/learn`.
- **`ON DELETE SET NULL`, not `CASCADE`** — when a cardgroup is deleted, the only meaning of `last_viewed_cardgroup_id` is "where to land you next" (presentation state). `CASCADE` would delete the user, which is absurd; `SET NULL` lets the HomePage redirect fall through to the next branch (cardgroups list, or onboarding).
- **`CREATE INDEX ... (last_viewed_cardgroup_id)`** — without an index, the cascading `SET NULL` on cardgroup delete forces a full users table scan. The index is on the dependent side, not the parent. Postgres does not auto-index the FK side; this is a known foot-gun on cascading deletes.

The existing `users` UPDATE RLS policy keys on `auth.uid() = id`, so the new column inherits the same row-level constraint without a policy edit. `backend/internal/repository/rls_user_test.go` covers cross-user UPDATE rejection on this column explicitly.

### Startup order

```
database.Migrate(url)
→ database.Open(ctx, cfg)
→ repository.NewUserRepository(db.GORM)
→ repository.NewRoleRepository(db.GORM)
→ repository.NewCardgroupRepository(db.GORM)
→ repository.NewPingRecordRepository(db.GORM)
→ server start
```

### Environment variables (database)

See also the general env-vars table above.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `SUPABASE_DB_URL` | yes | — | Supabase Postgres DSN (`postgres://...?sslmode=require`) |
| `DB_MAX_CONNS` | no | `10` | Maximum pool connections |
| `DB_MIN_CONNS` | no | `0` | Minimum pool connections kept alive |
| `DB_MAX_CONN_LIFETIME` | no | `30m` | Maximum lifetime of a pooled connection |
| `DB_MAX_CONN_IDLE_TIME` | no | `5m` | Maximum idle time before a connection is evicted |

