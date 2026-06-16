BEGIN;

-- Make master_cards.front case-insensitive so the Notion dictionary sync treats
-- "Drive" and "drive" as the same headword. citext compares case-insensitively
-- while preserving the stored case for display.
--
-- PRECONDITION: master_cards must contain no case-duplicate fronts within a
-- cardgroup, or the unique-index rebuild below fails. Resolve duplicates at the
-- source first (merge the Notion entries, then run the sync so the
-- case-sensitive prune removes the orphaned rows) and confirm this returns zero
-- rows before applying:
--   SELECT master_cardgroup_id, lower(front)
--   FROM public.master_cards
--   GROUP BY master_cardgroup_id, lower(front) HAVING count(*) > 1;

-- citext is installed without an explicit schema for portability: the local test
-- Postgres has no `extensions` schema, while `public` is on the search_path in
-- both environments. (A Supabase "extension in public" advisory may follow;
-- address it separately if the project relocates extensions to a dedicated
-- schema.)
CREATE EXTENSION IF NOT EXISTS citext;

-- Changing the column type rebuilds the dependent unique index
-- (uq_master_cards_cg_front) with citext's case-insensitive operator class, so
-- the ON CONFLICT (master_cardgroup_id, front) upsert becomes case-insensitive
-- with no index changes here. The front-length CHECK is re-validated and still
-- holds (char_length / trim work on citext).
ALTER TABLE public.master_cards
    ALTER COLUMN front TYPE citext USING front::citext;

COMMIT;
