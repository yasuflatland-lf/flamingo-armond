BEGIN;

-- Revert master_cards.front to case-sensitive text. The dependent unique index
-- (uq_master_cards_cg_front) rebuilds with text's default case-sensitive
-- operator class.
--
-- The citext extension is intentionally left installed: the up migration created
-- it with IF NOT EXISTS and may not have been the entity that first enabled it,
-- so dropping it here could remove a shared or pre-existing extension.
ALTER TABLE public.master_cards
    ALTER COLUMN front TYPE text USING front::text;

COMMIT;
