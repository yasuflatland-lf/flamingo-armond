-- Reverse of 20260927000004_drop_user_card_fsrs_last_rating.up.sql: restores
-- the column as nullable smallint only. The swipe_records backfill UPDATE from
-- 20260719000000_add_last_rating_to_user_card_fsrs is intentionally not re-run;
-- nothing reads the column, so the restored rows stay NULL.

BEGIN;

ALTER TABLE public.user_card_fsrs ADD COLUMN last_rating smallint;

COMMIT;
