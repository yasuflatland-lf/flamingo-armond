-- Drop user_card_fsrs.last_rating, added by
-- 20260719000000_add_last_rating_to_user_card_fsrs. Its only reader was the
-- rescue/filler learn-window predicate, which has been removed; the column has
-- had writers but no readers since.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_card_fsrs DROP COLUMN last_rating;

COMMIT;
