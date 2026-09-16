-- 20260913000000_lower_new_card_ratio_default.down.sql
--
-- Restores the historical new_card_ratio_num default. Existing rows are not
-- touched.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  ALTER COLUMN new_card_ratio_num SET DEFAULT 4;

COMMIT;
