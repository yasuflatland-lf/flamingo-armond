-- Add Notion document-order position to public.cards.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.cards
    ADD COLUMN IF NOT EXISTS position integer NOT NULL DEFAULT 0;

COMMIT;
