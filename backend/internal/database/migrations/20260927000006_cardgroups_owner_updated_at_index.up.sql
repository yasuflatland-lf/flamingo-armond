-- Index hygiene for public.cardgroups.
--
-- 1. Add (owner_id, updated_at DESC, id DESC).
--    myCardgroupsConnection (repository.cardgroupRepo.FindPageByOwner) always
--    filters WHERE owner_id = ? and orders by the fixed (updated_at DESC,
--    id DESC) with a cursor predicate on the same tuple. The composite serves
--    that page as an index-ordered LIMIT scan.
--
-- 2. Drop idx_cardgroups_updated_at.
--    No query filters or orders by updated_at without an owner_id predicate.
--    For an owner with many decks the planner still picks this index, as a
--    backward scan over every owner's rows filtered by owner_id plus an
--    incremental sort on id; the composite replaces exactly that plan.
--
-- 3. Drop idx_cardgroups_owner_id.
--    Every owner_id-equality access path (FindPageByOwner, CountByOwner,
--    CountByOwnerTx, the cardgroups_all_owner_or_admin RLS policy, the
--    users -> cardgroups ON DELETE CASCADE) is served by the new composite's
--    leading column.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_cardgroups_owner_updated_at_id
    ON public.cardgroups (owner_id, updated_at DESC, id DESC);

DROP INDEX IF EXISTS public.idx_cardgroups_updated_at;
DROP INDEX IF EXISTS public.idx_cardgroups_owner_id;

COMMIT;
