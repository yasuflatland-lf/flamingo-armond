-- 20260927000002_reset_legacy_new_card_ratio.down.sql
--
-- Reverse of 20260927000002_reset_legacy_new_card_ratio.up.sql: intentionally a
-- no-op. The up migration overwrote every 4/5 ratio with 1/5, and nothing
-- records which rows held 4/5 (or which non-reduced form of it) beforehand, so
-- the previous values are not reconstructible. Rewriting every 1/5 row back to
-- 4/5 would also change learners who were already at 1/5 before the up
-- migration ran.

DO $$
BEGIN
    RAISE NOTICE '20260927000002_reset_legacy_new_card_ratio: down is a no-op; reset ratios are not restored';
END
$$;
