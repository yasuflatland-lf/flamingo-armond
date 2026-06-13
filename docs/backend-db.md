# Backend database

> Pool / interface design, migrations (golang-migrate), `schema_migrations` and RLS, `SECURITY DEFINER` helper recipe, startup ordering, env vars. See `docs/backend.md` for runtime, [`docs/playbook-patterns.md` § "Recovering from a dirty migration"](playbook-patterns.md#recovering-from-a-dirty-migration) for incident recovery, and `.claude/rules/go-library-gotchas.md` for GORM-specific quirks.

## Database

### Pool and interface design

A single `pgxpool.Pool` is created at startup. `stdlib.OpenDBFromPool` converts it into a `*sql.DB`, which is then handed to GORM. The result is **one pool, two interfaces** — pgx native for low-level queries and GORM for the ORM layer — without double-consuming Render free tier's connection limit.

### Migrations

Migrations use `golang-migrate` with `*.up.sql` / `*.down.sql` files. Raw SQL lets you express triggers, foreign keys, and `SECURITY DEFINER` functions directly, none of which GORM's AutoMigrate can model. AutoMigrate is therefore not used.

Migration files live under `backend/internal/database/migrations/`. Go's `//go:embed` directive does not allow `..` path components, so the migration directory must sit inside the package tree rather than at the repo root.

**Filename format: `yyyymmddhhmmss_<short_snake_case_description>.{up,down}.sql`** — the 14-digit timestamp prefix is the numeric version `golang-migrate` records in `public.schema_migrations` and uses to order files. New migrations therefore need a timestamp strictly greater than every existing file (UTC is fine; the values just need to sort correctly). The trailing description is for human readers and is not parsed — pick a short verb-led summary like `create_cards`, `enable_rls_deny_all`, or `initial_schema`. Up and down halves must share the same prefix and description so `golang-migrate` can pair them.

When a migration fails mid-run, `schema_migrations.dirty=true` is set. Recovery requires an operator to run `migrate force <version>`. The `run()` function treats any migration error as fatal and returns immediately (fail-fast). See [`docs/playbook-patterns.md` § "Recovering from a dirty migration"](playbook-patterns.md#recovering-from-a-dirty-migration) for the operator runbook.

**`UPDATE schema_migrations SET dirty = false` alone is not enough.** A failed up migration leaves both `dirty = true` AND `version` pointing at the failed migration. Clearing only the dirty flag leaves the version pointing at the failed file, so the next `Steps(1)` call goes looking for migration `<failed+1>` and fails with `os.ErrNotExist`. The idiomatic recovery is `m.Force(<predecessor_version>)` — it rewrites both fields atomically and lets `Steps(1)` re-apply the original migration. The same gotcha applies to test code that simulates a dirty state via the migrate Go API; see `MigrateForceForTest` in `internal/database/export_test.go`.

#### golang-migrate transaction behaviour

**The `pgx/v5` driver does NOT auto-wrap each migration file in a transaction.** Every migration that requires atomicity must open its own `BEGIN; ... COMMIT;` block explicitly. A migration file that omits `BEGIN/COMMIT` and mixes DDL with privilege-sensitive statements (e.g. `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`) can partially succeed: the DDL commits, the later statement fails, and `schema_migrations.dirty=true` persists because golang-migrate commits that flag in its own separate transaction *before* it begins executing the migration SQL.

The structural consequence: **sensitive ALTER operations (RLS, GRANT, REVOKE) belong in their own dated migration file**, separate from the DDL that creates the tables. A failure in one file leaves the other file's work untouched, limiting blast radius.

#### Migration test quality bar

Tests that invoke the migration runner must assert *post-conditions*, not just "migrate ran without error". The model is `TestMigrations_AllPublicTablesHaveRLSEnabled` in `internal/database/pool_test.go`: it queries `pg_class` after migration and asserts every expected table has `relrowsecurity = true`. RLS policy behavior is covered by `internal/database/rls_test.go`, which connects as the Supabase-style `authenticated` role and sets local JWT claims before querying. Procedural success alone does not verify security posture.

**Every migration that adds an RLS policy must add a corresponding sub-test to `internal/database/rls_test.go`.** The sub-test must cover three cases: the owning user can read their own rows, a different user cannot read those rows, and the admin user can read any user's rows. For tables that restrict writes (INSERT/UPDATE), add cases that confirm permitted writes succeed and cross-user writes are denied. The `swipe_records` and `user_card_fsrs` sub-tests in `TestRLSPolicies_AuthenticatedRole` are the canonical models. A migration that enables RLS without a companion test leaves the policy behavior unverified at the code level.

**Every new migration also shifts the down/up roundtrip harness.** Per-migration roundtrip tests hardcode a relative `m.Steps(-N)` count that must be bumped by 1 for each migration newer than their target, and the generic `TestMigrateDownUpRoundtrip` hardcodes a `want` list of public tables that must gain any new table. See [Migration down/up roundtrip test § "Bump every per-migration `Steps(-N)`"](backend/library-gotchas/migration-down-up-roundtrip-test.md#bump-every-per-migration-steps-n-when-a-newer-migration-lands) for the harness (`grep -rn 'm.Steps(-' backend/internal/database/*_test.go` + the `want`-list update).

#### `schema_migrations` and RLS

`public.schema_migrations` is golang-migrate's internal bookkeeping table. It lives in the `public` schema, which Supabase exposes through PostgREST, so the Supabase-default GRANTs let the `anon` and `authenticated` roles read **and write** it over the REST API unless RLS intervenes — an integrity risk, since a caller could corrupt the recorded version or set `dirty=true` and disrupt deploys. The "owner bypasses RLS" property protects only the backend's and golang-migrate's own owner-role connections; it says nothing about the PostgREST `anon` / `authenticated` path, which is the real exposure.

The table therefore carries **deny-all RLS** — `ENABLE ROW LEVEL SECURITY` with no policy, plus an explicit `REVOKE ALL ... FROM anon, authenticated` (migration `20260603090000_enable_rls_schema_migrations`). `FORCE ROW LEVEL SECURITY` is deliberately **not** set, so the table owner (golang-migrate, and the backend) keeps bypassing RLS and version bookkeeping is unaffected. The resulting "RLS enabled, no policy" state is the intended posture for an internal table; it surfaces as a benign `rls_enabled_no_policy` **INFO** in the Supabase advisor rather than the `rls_disabled_in_public` **ERROR** the missing RLS previously raised. `TestMigrations_AllPublicTablesHaveRLSEnabled` asserts every public table — `schema_migrations` included — has RLS enabled.

**Renaming or renumbering migration files is not transparent to the DB.** `golang-migrate` records the numeric version of each applied migration in `public.schema_migrations`. Renaming a file (e.g. `0001_create_profiles.up.sql` → `20250101000000_create_profiles.up.sql`) rewrites the source tree but **not** the DB row, so the next boot fails with `no migration found for version <N>: read down for version <N> migrations: file does not exist` — migrate's source-state reconciliation expects the recorded version to exist on disk. When the rename is identifier-only (up/down SQL bodies are byte-identical, `git log -M` reports an `R100` rename), the safe recovery is `UPDATE public.schema_migrations SET version = <new_version>, dirty = false WHERE version = <old_version>` against the production DB; the runbook lives in `playbooks/setup-prod/recover-migration-version-rebase.sql`. Do **not** apply this shortcut when the rename also changed migration content — in that case, squash the changes and use `migrate force <version>` against a known-good source state so the new content actually runs.

#### SECURITY DEFINER helper recipe

`SECURITY DEFINER` SQL functions used by RLS policies (e.g. `private.is_admin(uid uuid)`) must replicate this exact shape — each attribute has a load-bearing reason:

```sql
CREATE SCHEMA IF NOT EXISTS private;

CREATE OR REPLACE FUNCTION private.is_admin(uid uuid)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$ ... $$;

-- authenticated reaches the helper from RLS policies; anon never does.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT USAGE ON SCHEMA private TO authenticated;
        GRANT EXECUTE ON FUNCTION private.is_admin(uuid) TO authenticated;
    END IF;
END
$$;
```

- `STABLE`, **not** `IMMUTABLE` — the function reads tables, and marking a table-reading function `IMMUTABLE` corrupts the planner's plan cache (the planner assumes the result is constant for fixed inputs).
- `SECURITY DEFINER` — the function executes as its owner, so RLS-enabled callers can probe role membership without needing direct read on `roles` / `user_roles`.
- `SET search_path = public` — neutralises the classic `SECURITY DEFINER` injection vector where an attacker creates a `pg_temp` shim function (e.g. their own `roles` table) that the function would otherwise resolve before the real one.
- **Lives in the `private` schema, not `public`.** PostgREST exposes only the `public` schema (plus any explicitly configured), so a helper in `private` is never reachable as a `/rest/v1/rpc/...` endpoint — this is what clears the `anon_security_definer_function_executable` and `authenticated_security_definer_function_executable` advisor warnings. RLS policies resolve the function by OID regardless of its schema, and the `authenticated` role is granted `USAGE` on `private` so policy evaluation can still call it; `anon` is granted nothing. `is_admin` was retrofitted into `private` by migration `20260603090200_restrict_definer_function_exposure` via `ALTER FUNCTION ... SET SCHEMA private` (OID-preserving, so the existing policies kept resolving untouched).

  **Policy-authoring consequence: a new admin-only RLS policy in any migration that runs after `20260603090200_restrict_definer_function_exposure` must reference `private.is_admin(auth.uid())`, never `public.is_admin(...)`.** Because that migration moved the function out of `public` (`ALTER FUNCTION public.is_admin(uuid) SET SCHEMA private`), a `CREATE POLICY ... USING (public.is_admin(auth.uid()))` fails at apply time with `function public.is_admin(uuid) does not exist` and leaves `schema_migrations.dirty = true`. The `master_cardgroups_admin_all` / `master_cards_admin_all` policies in `20260614000000_add_master_tables` reference `private.is_admin(auth.uid())` for exactly this reason.
- `DO $$ ... IF EXISTS pg_roles ... GRANT END $$` portability guard — plain PostgreSQL does not include Supabase-managed roles (`authenticated`, `anon`, `service_role`) by default. Wrapping role-specific GRANTs in this conditional `DO` block keeps the migration applicable outside Supabase. Testcontainers create a minimal `authenticated` role fixture so RLS behavior can be exercised directly. The Go backend connects as the table owner and bypasses RLS, so the GRANT path is used only by direct PostgREST / Edge callers in production.

Trigger functions such as the `set_*_updated_at` family are `SECURITY INVOKER` and need neither the `private`-schema move nor GRANT management, but they MUST still pin `SET search_path = ''` to satisfy the `function_search_path_mutable` advisor (migration `20260603090100_pin_trigger_function_search_path`).

**New trigger / `SECURITY DEFINER` functions created in a migration must bake in the hardening from creation** — the correct `SECURITY INVOKER` / `SECURITY DEFINER` mode plus the pinned `SET search_path` — rather than creating an un-hardened function and relying on a later retrofit migration. The existing function set was retrofitted twice (`20260603090100_pin_trigger_function_search_path` pinned the trigger search paths; `20260603090200_restrict_definer_function_exposure` moved `is_admin` into `private` and revoked the API-role grants); a migration that re-introduces an un-hardened function only re-raises the advisor warnings those two migrations cleared. New functions START hardened. Worked example: `set_master_cardgroups_updated_at` / `set_master_cards_updated_at` in `20260614000000_add_master_tables` are declared `SET search_path = ''` and `SECURITY INVOKER` (the default) at creation, with no follow-up retrofit needed.

The same recipe applies to functions invoked by Supabase GoTrue at JWT mint time (the Custom Access Token Hook). See [`docs/backend/custom-access-token-hook.md`](backend/custom-access-token-hook.md) for the hook-specific design decisions — join-at-mint over sync-trigger, fail-closed on malformed events, stale-claim removal, canonical return shape, and the down-migration operator precondition.

### Index strategy

Every foreign key in the schema has a backing btree index so that `ON DELETE CASCADE` lookups never degrade to a sequential scan. A composite primary key only backs lookups on its left-prefix columns: `PRIMARY KEY (user_id, card_id)` does NOT serve a lookup by `card_id` alone, so `user_card_fsrs.card_id` carries a dedicated `idx_user_card_fsrs_card_id` index (migration `20260616000000_add_user_card_fsrs_card_id_index`).

The following read-path composite indexes are intentionally **not** present. They are pagination tuning whose absence degrades gracefully (a page sorts more rows) and which can be added later with a single migration once load is measured. Add them — preferably with `CREATE INDEX CONCURRENTLY` in a file without `BEGIN/COMMIT` if the target table is already large in production — when the trigger fires:

- `cards (cardgroup_id, id)` — `cardsByCardgroupConnection` defaults to `ORDER BY id` within a `cardgroup_id`, which currently filters via `idx_cards_cardgroup_id` and then sorts. Add when a single cardgroup routinely holds several thousand cards, or when `EXPLAIN (ANALYZE, BUFFERS)` shows a Sort node dominating the page query. `cards` is on the bulk-import (upsert) write path, so this index is not added pre-emptively — it would tax every insert for a read that is not yet slow. If added, also drop the now-redundant `idx_cards_cardgroup_id` (its `cardgroup_id` prefix is covered by the composite).
- `users (created_at DESC, id ASC)` — the admin user list orders by `(created_at DESC, id ASC)`; no current index supports that mixed-direction order, so each page sorts the table. Add when the admin list is perceptibly slow, or when `users` reaches tens of thousands of rows.

Measure before adding either: `EXPLAIN (ANALYZE, BUFFERS)` on the query plus `pg_stat_user_tables.seq_scan` / `pg_stat_statements` to confirm the index will be used.

### Postgres upsert: prerequisite UNIQUE / EXCLUSION constraint

`INSERT ... ON CONFLICT (cols) ...` requires the column set to be backed by a UNIQUE constraint, UNIQUE INDEX, or EXCLUSION constraint. Plain (non-unique) indexes and CHECK constraints do not satisfy the requirement — Postgres rejects the statement with SQLSTATE `42P10` "there is no unique or exclusion constraint matching the ON CONFLICT specification". This is checked at planning time, so the failure surfaces immediately, not on a colliding row.

The repository's two upsert call sites are deliberately asymmetric. `cards (cardgroup_id, front)` is backed by `uq_cards_cardgroup_front` (added in `20260503000000_add_cards_upsert_index`) because the dictionary upsert pipeline requires the constraint to dedup imports. `cardgroups (owner_id, name)` is **not** backed by any UNIQUE — the product allows the same user to create two cardgroups with the same name, and the GraphQL mutation validates only the name's length. A caller (production code or test helper) that issues `ON CONFLICT (owner_id, name)` against `cardgroups` is therefore invalid by schema design, not by a missing constraint that ought to be added.

### Postgres upsert: classifying inserted vs updated rows in one round-trip

`INSERT ... ON CONFLICT (key) DO UPDATE ... RETURNING (xmax = 0) AS inserted` lets a single statement report which rows were inserted and which were updated. PostgreSQL marks freshly inserted rows with `xmax = 0` and ON-CONFLICT-updated rows with `xmax = current_xid`, so the boolean expression in `RETURNING` classifies each row inline — no second `SELECT`, no application-side bookkeeping. The `cards` upsert path uses this in `cardRepo.UpsertManyTx` to compute the `inserted` / `updated` split.

The flip side is that the conflict key MUST be unique within the input batch. If two rows in the same `INSERT ... VALUES (...), (...)` collide on the conflict target, Postgres raises SQLSTATE `21000` ("ON CONFLICT DO UPDATE command cannot affect row a second time") and aborts the whole statement — Postgres deliberately does not silently merge intra-batch duplicates because either-row-wins is non-deterministic. The application layer must dedup by conflict key before issuing the SQL; the dictionary usecase keeps the *last* occurrence and reports earlier ones as soft errors.

### One-time master-catalog migration CLI (`cmd/migrate-to-master`)

`backend/cmd/migrate-to-master` copies an owner's existing personal deck
(`public.cardgroups` / `public.cards`) into the admin-curated master catalog
tables (`public.master_cardgroups` / `public.master_cards`). The personal rows
are **kept** — the tool never deletes them, so the owner's FSRS history
(`public.user_card_fsrs`, keyed by card id) is preserved. Each personal
cardgroup is snapshotted as a published default starter deck
(`status='published'`, `is_default_starter=true`, `source='notion'`,
`version=1`, `sort_order=0`); the source primary key is preserved, so re-runs
converge via `ON CONFLICT (id) DO UPDATE` (idempotent).

The tool is built like the other single-purpose CLIs — raw `database/sql` +
the pgx stdlib driver, validate the DSN before `sql.Open`, explicit
`rows.Close()` after each loop in addition to `defer` (see
[`docs/backend/library-gotchas/raw-sql-cli-pgx-stdlib.md`](backend/library-gotchas/raw-sql-cli-pgx-stdlib.md)).
It requires the **superuser DSN** because resolving `--owner-email` reads
`auth.users`, which is owned by `supabase_auth_admin` and not readable by the
application role.

Two subcommands:

```bash
# Read-only parity report: master_cardgroups/master_cards vs the owner's deck.
go run ./cmd/migrate-to-master verify --db-url "$SUPERUSER_DSN" --owner-email owner@example.com

# Copy the deck into the master tables (one all-or-nothing transaction).
go run ./cmd/migrate-to-master run    --db-url "$SUPERUSER_DSN" --owner-email owner@example.com
```

**Runbook — running against production is a destructive DB action and requires
explicit operator confirmation at execution time:**

1. **Staging verify** — run `verify` against staging to confirm the master
   tables exist and read access works. It exits non-zero until the deck has
   been copied, which is expected pre-`run`.
2. **Snapshot** — take a production database backup/snapshot before any write.
3. **Prod run** — run `run` against production. The upserts are wrapped in a
   single transaction; a failure rolls back with no partial copy.
4. **Prod verify** — run `verify` against production. Parity (`personal == master`
   for the owner's row ids) confirms the copy landed.
5. **Rollback** — because `run` only inserts/updates master rows keyed by the
   source ids (and never touches `public.cardgroups` / `public.cards`),
   rollback is `DELETE FROM public.master_cards WHERE id = ANY(<copied card ids>)`
   followed by `DELETE FROM public.master_cardgroups WHERE id = ANY(<copied cardgroup ids>)`
   (the `master_cards` FK is `ON DELETE CASCADE`, so deleting the cardgroups
   alone also removes their cards). Restoring the pre-`run` snapshot is the
   coarser fallback.

A throwaway-DB `run`-then-`verify` parity test (`cmd/migrate-to-master/main_test.go`,
testcontainers) asserts the copy lands, is idempotent, and leaves the personal
rows untouched; full integration runs against a real Supabase instance go to CI.

### `user_preferences` — per-user UI continuity

`public.user_preferences` is a sibling aggregate of `public.users`, keyed 1:1 by `user_id`. It carries presentation state that is **about** the user but not **part of** their identity (currently `last_viewed_cardgroup_id`; future fields like `theme`, `default_mode`, ... extend cleanly on this table). The extraction rationale and structural signals are documented in [`docs/backend/library-gotchas/sibling-aggregate-extraction.md`](backend/library-gotchas/sibling-aggregate-extraction.md).

Schema (from `20260516120000_extract_user_preferences.up.sql`):

```sql
CREATE TABLE public.user_preferences (
    user_id                  uuid PRIMARY KEY
        REFERENCES public.users(id) ON DELETE CASCADE,
    last_viewed_cardgroup_id uuid NULL
        REFERENCES public.cardgroups(id) ON DELETE SET NULL,
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_preferences_last_viewed_cardgroup_id
    ON public.user_preferences (last_viewed_cardgroup_id);
```

Four design choices are load-bearing:

- **Lazy row creation via `INSERT ... ON CONFLICT (user_id) DO UPDATE`.** "Row exists = user has set a preference" is a clean semantic — no need for an `auth.users` trigger to seed empty rows, and future preference columns inherit "no row = all defaults" for free. The repository's UPSERT statement also carries an ownership predicate, see [Ownership-checked UPSERT](backend/library-gotchas/ownership-checked-upsert-where-exists.md).
- **`user_id ... ON DELETE CASCADE`.** When the owning user is deleted, the preference row must follow — `RESTRICT` would block the user-account-deletion flow, and `SET NULL` is impossible on a `PRIMARY KEY` column. Verified by `TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow` (see [FK action integration test](backend/library-gotchas/fk-action-integration-test.md)).
- **`last_viewed_cardgroup_id ... ON DELETE SET NULL`, not `CASCADE`.** When a cardgroup is deleted, the only meaning of `last_viewed_cardgroup_id` is "where to land you next" (presentation state). `CASCADE` would delete the preference row entirely — and on the next visit the HomePage redirect would miss the chance to fall through gracefully to the cardgroups list. `SET NULL` keeps the row and lets the resolver return `nil` for the field. Verified by `TestUserPreferenceRepository_OnDeleteCardgroup_SetsNull`.
- **`CREATE INDEX ... (last_viewed_cardgroup_id)`.** Without an index, the cascading `SET NULL` on cardgroup delete forces a full preferences table scan. The index is on the dependent side, not the parent. Postgres does not auto-index the FK side; this is a known foot-gun on cascading deletes.

RLS policies on `user_preferences` mirror the `users` shape (self-or-admin SELECT / INSERT / UPDATE / DELETE). The four policies are declared in the up migration and dropped implicitly by the down migration's `DROP TABLE`. The INSERT-own policy regression test reserves a fresh fixture user to avoid the `ON CONFLICT DO NOTHING` short-circuit — see [RLS `INSERT-own` assertion needs a fresh fixture row](backend/library-gotchas/rls-insert-own-fresh-fixture-row.md). The down/up migration roundtrip (including the NULL-pref edge case) is covered by `TestExtractUserPreferences_DownUpRoundtrip` and `TestExtractUserPreferences_NullPrefHandledByDown`; the general pattern lives in [Migration down/up roundtrip test](backend/library-gotchas/migration-down-up-roundtrip-test.md).

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

