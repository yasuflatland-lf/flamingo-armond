-- 20260727000000_realign_fsrs_snapshot_columns_to_v4.down.sql
--
-- Reverse of 20260727000000_realign_fsrs_snapshot_columns_to_v4.up.sql.
-- elapsed_days returns as 0 because the removed value is not reconstructible.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  DROP COLUMN IF EXISTS due_before,
  ALTER COLUMN phase_before DROP NOT NULL,
  ALTER COLUMN stability_before DROP NOT NULL,
  ADD COLUMN IF NOT EXISTS scheduled_days_before integer,
  ADD COLUMN IF NOT EXISTS elapsed_days integer NOT NULL DEFAULT 0;
ALTER TABLE public.swipe_records ALTER COLUMN elapsed_days DROP DEFAULT;

ALTER TABLE public.user_card_fsrs
  ADD COLUMN IF NOT EXISTS elapsed_days integer NOT NULL DEFAULT 0;
ALTER TABLE public.user_card_fsrs ALTER COLUMN elapsed_days DROP DEFAULT;

COMMIT;
