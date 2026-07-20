-- 20260721000000_add_cardgroup_fk_to_swipe_records.down.sql
--
-- Reverse of 20260721000000_add_cardgroup_fk_to_swipe_records.up.sql: drops both
-- the foreign key and the index that backs it. Rows deleted by the up
-- migration's orphan cleanup are not restored -- they were unreachable garbage.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
    DROP CONSTRAINT IF EXISTS swipe_records_cardgroup_id_fkey;

DROP INDEX IF EXISTS public.idx_swipe_records_cardgroup_id;

COMMIT;
