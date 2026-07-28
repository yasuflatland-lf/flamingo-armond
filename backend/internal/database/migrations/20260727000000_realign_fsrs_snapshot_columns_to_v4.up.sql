-- 20260727000000_realign_fsrs_snapshot_columns_to_v4.up.sql
--
-- Realigns the persisted FSRS snapshot to go-fsrs v4's elapsed-day model.
--
--   DROP user_card_fsrs.elapsed_days, swipe_records.elapsed_days
--     v4 removed Card.ElapsedDays. The application synthesised the value, and its
--     only reader was the on-time-recall statistic, which now compares instants.
--   DROP swipe_records.scheduled_days_before
--     Its readers were the interval heuristic and the on-time day comparison, both
--     replaced by due_before.
--   ADD swipe_records.due_before
--     The due instant the card carried going into the review -- the boundary the
--     review is actually judged against.
--   SET NOT NULL on phase_before, stability_before, due_before
--     The NULL sentinel marked rows recorded before those columns existed. The
--     branches that read it are gone, so the sentinel is now only a hole.
--
-- The NOT NULL step cannot be backfilled. 20260718 records that deriving a
-- pre-swipe phase from the stored post-swipe state is provably ambiguous, and a
-- pre-swipe due cannot be reconstructed at all. Existing swipe history is
-- therefore deleted; it feeds only the /stats diagnostics.
--
-- user_card_fsrs rows are NOT deleted -- only a column is dropped, so every card
-- keeps its schedule and no learner loses progress.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DELETE FROM public.swipe_records;

ALTER TABLE public.swipe_records
  DROP COLUMN IF EXISTS elapsed_days,
  DROP COLUMN IF EXISTS scheduled_days_before,
  ADD COLUMN due_before timestamptz NOT NULL,
  ALTER COLUMN phase_before SET NOT NULL,
  ALTER COLUMN stability_before SET NOT NULL;

ALTER TABLE public.user_card_fsrs
  DROP COLUMN IF EXISTS elapsed_days;

COMMIT;
