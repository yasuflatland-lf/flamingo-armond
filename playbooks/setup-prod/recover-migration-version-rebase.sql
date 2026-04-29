-- recover-migration-version-rebase.sql
--
-- One-shot recovery script for the "renumbered migration files" trap:
-- when files in backend/internal/database/migrations/ have been renamed
-- but the production DB's public.schema_migrations row still points at
-- the OLD identifier, the next backend boot dies with:
--
--   no migration found for version <N>: read down for version <N>
--   migrations: file does not exist
--
-- Symptom in Render: "Exited with status 1 while running your code"
-- with the above error in deploy logs at startup.
--
-- This script ONLY applies when the rename was identifier-only (the
-- up/down SQL bodies are byte-identical, `git log -M` reports an R100
-- rename). If the migration body was edited as part of the rename, do
-- not run this script; squash the changes and use `migrate force
-- <version>` against a known-good source state instead.
--
-- Run via the Supabase SQL editor (or `psql` against SUPABASE_DB_URL).
-- The four steps are intentionally separate so an operator can stop
-- after STEP 1 if the diagnostic does not match.
--
-- See also:
--   docs/backend.md  § "Migrations"
--   docs/deployment.md  § "Operational gotchas"


-- ============================================================
-- STEP 1 — Diagnose current state (read-only)
-- ============================================================
-- Expected output:
--   * Exactly one row in schema_migrations whose `version` matches
--     the OLD identifier (e.g. 1) and `dirty = false`.
--   * `version` column data type is `bigint` so the new value below
--     fits without overflow (20250101000000 > 2^31).
-- If `dirty = true`, the failure is a mid-migration crash, not a
-- rename trap: stop and use `migrate force <version>` per
-- docs/backend.md § "Migrations".

SELECT * FROM public.schema_migrations;

SELECT column_name, data_type
  FROM information_schema.columns
 WHERE table_schema = 'public'
   AND table_name   = 'schema_migrations';


-- ============================================================
-- STEP 2 — Confirm byte-identity in the source tree
-- ============================================================
-- Before running STEP 3, list the migration files in your local
-- checkout:
--
--   ls backend/internal/database/migrations/*.up.sql
--
-- Pick the EARLIEST identifier in the new naming scheme that is
-- byte-identical to what was applied as the OLD identifier:
--
--   git log --follow --diff-filter=R --name-status -- \
--     'backend/internal/database/migrations/*.up.sql'
--
-- The rename commit must show R100 for the file pair. If it shows
-- R<100 (any content change) or A/D (added/deleted, not renamed),
-- stop and use `migrate force` instead — STEP 3 below will skip
-- migrations that were never re-applied.


-- ============================================================
-- STEP 3 — Rebase schema_migrations to the new identifier
-- ============================================================
-- Replace the two literals below with the actual values confirmed
-- in STEP 1 (from_version) and STEP 2 (to_version). The transaction
-- wrapper makes a typo recoverable: review the SELECT after the
-- UPDATE, then change ROLLBACK to COMMIT only if it matches.

BEGIN;

UPDATE public.schema_migrations
   SET version = 20250101000000,  -- to_version (from STEP 2)
       dirty   = false
 WHERE version = 1;                -- from_version (from STEP 1)

-- Verify exactly one row was updated. Expected: a single row with
-- the new version and dirty=false. If the row count is 0 or > 1,
-- keep the ROLLBACK below and re-check STEP 1.
SELECT * FROM public.schema_migrations;

ROLLBACK;
-- Replace the ROLLBACK above with COMMIT after manual review:
-- COMMIT;


-- ============================================================
-- STEP 4 — Trigger a redeploy
-- ============================================================
-- After COMMIT in STEP 3, redeploy via:
--   Render dashboard → flamingo-backend → Manual Deploy →
--     "Deploy latest commit"
--
-- The startup log should now show the remaining migrations
-- applying in order, ending with "server starting addr=:1323".
-- If the boot still fails with the same error, re-run STEP 1
-- and confirm the UPDATE actually committed.
