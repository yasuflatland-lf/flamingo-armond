-- 20260913000000_lower_new_card_ratio_default.up.sql
--
-- Lowers the column default for new_card_ratio_num from 4 to 1 so a user row
-- created without an explicit ratio starts at 1/5 (4 new + 16 review per
-- 20-card session), matching domain.DefaultNewCardRatio. new_card_ratio_den
-- stays 5. Existing rows are not touched: a stored ratio is the learner's
-- choice.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  ALTER COLUMN new_card_ratio_num SET DEFAULT 1;

COMMIT;
