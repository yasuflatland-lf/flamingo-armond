-- 20260705000000_add_new_card_ratio_to_user_preferences.up.sql
--
-- Adds the per-user new-vs-review interleave ratio to public.user_preferences,
-- held as an irreducible fraction new_card_ratio_num / new_card_ratio_den.
-- The default 4/5 (new share 4, review share 1 -> interleave 4:1) keeps the
-- ordering behaviour identical to the historical fixed 4:1 constants.
--
-- The CHECK is a backstop for direct DB writes; the domain value object
-- enforces the same bounds plus gcd-reduction (which the CHECK cannot express)
-- in application code.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  ADD COLUMN new_card_ratio_num int NOT NULL DEFAULT 4,
  ADD COLUMN new_card_ratio_den int NOT NULL DEFAULT 5,
  ADD CONSTRAINT user_preferences_new_card_ratio_check
    CHECK (
      new_card_ratio_num >= 1
      AND new_card_ratio_num < new_card_ratio_den
      AND new_card_ratio_den <= 100
    );

COMMIT;
