-- 20260615000000_add_learn_display_mode_to_user_preferences.down.sql
--
-- Reverse of 20260615000000_add_learn_display_mode_to_user_preferences.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  DROP COLUMN IF EXISTS learn_display_mode;

COMMIT;
