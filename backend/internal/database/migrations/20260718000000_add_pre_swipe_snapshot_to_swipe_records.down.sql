-- 20260718000000_add_pre_swipe_snapshot_to_swipe_records.down.sql
--
-- Reverse of 20260718000000_add_pre_swipe_snapshot_to_swipe_records.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  DROP COLUMN IF EXISTS phase_before,
  DROP COLUMN IF EXISTS scheduled_days_before;

COMMIT;
