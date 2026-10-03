-- Reverse of 20260927000008_drop_swipe_records_after_state.up.sql: restores the
-- seven columns with the initial schema's types and NOT NULL. The dropped values
-- are not reconstructible, so existing rows get placeholder defaults, which are
-- then removed to match the original column definitions.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
  ADD COLUMN due            timestamptz      NOT NULL DEFAULT now(),
  ADD COLUMN stability      double precision NOT NULL DEFAULT 0,
  ADD COLUMN scheduled_days integer          NOT NULL DEFAULT 0,
  ADD COLUMN reps           integer          NOT NULL DEFAULT 0,
  ADD COLUMN lapses         integer          NOT NULL DEFAULT 0,
  ADD COLUMN state          integer          NOT NULL DEFAULT 0,
  ADD COLUMN last_review    timestamptz      NOT NULL DEFAULT now();
ALTER TABLE public.swipe_records
  ALTER COLUMN due            DROP DEFAULT,
  ALTER COLUMN stability      DROP DEFAULT,
  ALTER COLUMN scheduled_days DROP DEFAULT,
  ALTER COLUMN reps           DROP DEFAULT,
  ALTER COLUMN lapses         DROP DEFAULT,
  ALTER COLUMN state          DROP DEFAULT,
  ALTER COLUMN last_review    DROP DEFAULT;

COMMIT;
