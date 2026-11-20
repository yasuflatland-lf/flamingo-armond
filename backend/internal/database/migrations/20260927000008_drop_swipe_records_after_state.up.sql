-- Drop the post-swipe FSRS state columns from swipe_records that nothing reads:
-- due, stability, scheduled_days, reps, lapses, state, last_review. The /stats
-- readers moved to the pre-swipe snapshot columns in
-- 20260727000000_realign_fsrs_snapshot_columns_to_v4; of the post-swipe values
-- only difficulty still has a reader (the avgDifficulty diagnostic), so it stays.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  DROP COLUMN due,
  DROP COLUMN stability,
  DROP COLUMN scheduled_days,
  DROP COLUMN reps,
  DROP COLUMN lapses,
  DROP COLUMN state,
  DROP COLUMN last_review;

COMMIT;
