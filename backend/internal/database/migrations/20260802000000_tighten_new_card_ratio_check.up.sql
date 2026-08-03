-- 20260802000000_tighten_new_card_ratio_check.up.sql
--
-- Narrows user_preferences_new_card_ratio_check to the bounds
-- domain.ParseNewCardRatio actually enforces. The previous CHECK allowed
-- den <= 100 and ignored the 80% new-share ceiling, so a direct DB write could
-- store a ratio the application then discarded on every read (falling back to
-- DefaultNewCardRatio) with no error surfaced anywhere.
--
-- The value object additionally requires gcd(num, den) == 1, which a CHECK
-- cannot express; that half stays application-only, as before.
--
-- NOT VALID: existing rows are deliberately left unscanned. Production data is
-- not being migrated, and a legacy out-of-range row already degrades safely to
-- DefaultNewCardRatio at read time. Validating the table would turn that benign
-- degradation into a failed deploy.
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
      AND new_card_ratio_den <= 20
      AND 20 % new_card_ratio_den = 0
      AND new_card_ratio_num * 5 <= new_card_ratio_den * 4
    ) NOT VALID;

COMMIT;
