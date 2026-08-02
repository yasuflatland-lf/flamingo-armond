-- 20260802000000_tighten_new_card_ratio_check.down.sql
--
-- Restores the looser CHECK installed by
-- 20260705000000_add_new_card_ratio_to_user_preferences.
--
-- NOT VALID: the forward migration deliberately left existing rows unscanned, so
-- a legacy row may fail even these looser bounds (for example den > 100).
-- Validating on the down path would turn a rollback into a failed migration, so
-- the reverse direction stays unscanned exactly like the forward one.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  DROP CONSTRAINT IF EXISTS user_preferences_new_card_ratio_check;

ALTER TABLE public.user_preferences
  ADD CONSTRAINT user_preferences_new_card_ratio_check
    CHECK (
      new_card_ratio_num >= 1
      AND new_card_ratio_num < new_card_ratio_den
      AND new_card_ratio_den <= 100
    ) NOT VALID;

COMMIT;
