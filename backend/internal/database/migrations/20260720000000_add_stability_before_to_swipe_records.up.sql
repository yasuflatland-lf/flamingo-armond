-- 20260720000000_add_stability_before_to_swipe_records.up.sql
--
-- Adds the pre-swipe FSRS stability snapshot to public.swipe_records:
--   stability_before -- the FSRS stability (days) the card held before this swipe
--
-- The column is nullable with NO default and NO backfill: NULL marks a row
-- recorded before it existed and is a meaningful sentinel the metrics layer
-- branches on. The stats "already learned" gate must test stability against
-- domain.LearnedStabilityDays, the same boundary the mastery tiles use;
-- scheduled_days_before is only an approximation of it, because go-fsrs clamps
-- each rating's interval to exceed the previous rating's, so a recorded interval
-- can outrun round(stability) by several days.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  ADD COLUMN stability_before double precision;

COMMIT;
