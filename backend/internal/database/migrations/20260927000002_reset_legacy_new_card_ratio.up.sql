-- 20260927000002_reset_legacy_new_card_ratio.up.sql
--
-- Rewrites every stored new-card ratio equal to 4/5 to the current default 1/5.
--
-- 20260705000000 added new_card_ratio_num / new_card_ratio_den as NOT NULL
-- DEFAULT 4/5, which filled every existing row, and every row created later by
-- the last-viewed-cardgroup or learn-display-mode upserts took the same default
-- without the learner choosing a ratio. 20260913000000 lowered the default to
-- 1/5 but left those rows at 4/5 on the premise that a stored ratio is the
-- learner's choice. No column records whether a ratio was chosen, so that
-- premise does not hold. A learner who did choose 80% cannot be told apart from
-- one who inherited it and is reset as well; the /profile setting can choose it
-- again.
--
-- The predicate matches every representation of 4/5, not only the reduced pair
-- the application writes: 8/10 and 16/20 satisfy the current CHECK, and rows
-- written under the looser 20260705000000 CHECK may hold 12/15 or other
-- multiples. The num > 0 guard keeps a 0/0 pair out of the cross-multiplied
-- equality. user_preferences has no updated_at trigger, so the statement sets
-- updated_at itself, as every repository write does.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

UPDATE public.user_preferences
SET new_card_ratio_num = 1,
    new_card_ratio_den = 5,
    updated_at         = now()
WHERE new_card_ratio_num > 0
  AND new_card_ratio_num * 5 = new_card_ratio_den * 4;

COMMIT;
