-- Reverse of 20260529090000_add_position_to_cards.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.cards DROP COLUMN IF EXISTS position;

COMMIT;
