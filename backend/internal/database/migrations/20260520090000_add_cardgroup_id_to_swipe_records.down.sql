-- Reverse of 20260520090000_add_cardgroup_id_to_swipe_records.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP INDEX IF EXISTS public.idx_swipe_records_user_cardgroup;

ALTER TABLE public.swipe_records
    DROP COLUMN IF EXISTS cardgroup_id;

COMMIT;
