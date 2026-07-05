-- 20260705000000_add_new_card_ratio_to_user_preferences.down.sql
--
-- Reverse of 20260705000000_add_new_card_ratio_to_user_preferences.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  DROP CONSTRAINT IF EXISTS user_preferences_new_card_ratio_check,
  DROP COLUMN IF EXISTS new_card_ratio_num,
  DROP COLUMN IF EXISTS new_card_ratio_den;

COMMIT;
