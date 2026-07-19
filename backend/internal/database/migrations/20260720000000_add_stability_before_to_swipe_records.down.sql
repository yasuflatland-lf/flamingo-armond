-- 20260720000000_add_stability_before_to_swipe_records.down.sql
--
-- Reverse of 20260720000000_add_stability_before_to_swipe_records.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  DROP COLUMN IF EXISTS stability_before;

COMMIT;
