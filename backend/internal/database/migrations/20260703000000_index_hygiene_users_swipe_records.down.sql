-- Reverse of 20260703000000_index_hygiene_users_swipe_records.up.sql.
--
-- Recreate the two dropped single-column indexes with their original
-- definitions from 20260430080000_initial_schema.up.sql, then drop the
-- composite (created_at DESC, id ASC) index the up migration added. Statements
-- run in the exact reverse order of the up migration.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_swipe_records_user_id
    ON public.swipe_records (user_id);

CREATE INDEX IF NOT EXISTS idx_users_updated_at
    ON public.users (updated_at);

DROP INDEX IF EXISTS public.idx_users_created_at_id;

COMMIT;
