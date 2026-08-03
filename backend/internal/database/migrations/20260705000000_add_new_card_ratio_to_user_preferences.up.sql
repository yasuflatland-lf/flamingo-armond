-- 20260705000000_add_new_card_ratio_to_user_preferences.up.sql
--
-- Adds the per-user new-vs-review interleave ratio to public.user_preferences,
-- held as an irreducible fraction new_card_ratio_num / new_card_ratio_den.
-- The default 4/5 (new share 4, review share 1 -> interleave 4:1) keeps the
-- ordering behaviour identical to the historical fixed 4:1 constants.
--
-- The CHECK here is a LOOSE backstop for direct DB writes: it bounds the
-- fraction's shape but not the domain value object's real limits (the reduced
-- denominator cap and the 80% new-card share ceiling), and it cannot express
-- gcd-reduction at all. Migration 20260802000000_tighten_new_card_ratio_check
-- supersedes it with the value object's actual bounds; gcd-reduction stays
-- application-only.
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
