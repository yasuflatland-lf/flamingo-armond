-- ping_records rows are only inserted (Create) and bulk-deleted (DeleteAll);
-- no UPDATE ever runs and no trigger maintains updated_at, so the column
-- always equals created_at and nothing reads it.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.ping_records DROP COLUMN IF EXISTS updated_at;

COMMIT;
