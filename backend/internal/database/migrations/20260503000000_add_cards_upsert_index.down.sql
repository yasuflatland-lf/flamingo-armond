-- Reverse of 20260503000000_add_cards_upsert_index.up.sql.
--
-- Dropping the unique index is sufficient: the pre-check in the up
-- migration only blocked migration on duplicates, it did not modify
-- existing rows, so there is nothing to restore on the data side.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction;
-- the explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP INDEX IF EXISTS public.uq_cards_cardgroup_front;

COMMIT;
