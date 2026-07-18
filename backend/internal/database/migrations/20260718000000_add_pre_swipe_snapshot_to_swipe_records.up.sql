-- 20260718000000_add_pre_swipe_snapshot_to_swipe_records.up.sql
--
-- Adds the pre-swipe FSRS snapshot to public.swipe_records:
--   phase_before          -- the FSRS phase the card was in before this swipe
--   scheduled_days_before -- the interval (days) scheduled at the previous review
--
-- Both columns are nullable with NO default and NO backfill: NULL marks a
-- legacy row recorded before these columns existed and is a meaningful sentinel
-- the metrics layer branches on. Deriving the pre-swipe phase from the stored
-- post-swipe state is provably ambiguous (a Learning->Easy graduation and a
-- Review->Hard review both land on phase Review), so the snapshot is captured
-- on the swipe event going forward.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  ADD COLUMN phase_before smallint,
  ADD COLUMN scheduled_days_before integer;

COMMIT;
