-- Index hygiene for public.users and public.swipe_records.
--
-- 1. Add public.users (created_at DESC, id ASC).
--    The admin users connection (repository.UserRepository.ListPage) always
--    orders by (created_at DESC, id ASC) with a cursor predicate on
--    (created_at, id), but the initial schema only indexed users.updated_at,
--    a column no repository query filters or orders by. Every admin users page
--    therefore did a sequential scan + top-N sort of public.users. The
--    composite (created_at DESC, id ASC) matches the cursor tuple order, so it
--    serves both forward pagination and backward pagination (via a backward
--    index scan) as an index-ordered LIMIT scan.
--
-- 2. Drop public.idx_users_updated_at.
--    No repository query filters or orders by users.updated_at; the index is
--    pure write overhead on every profile update.
--
-- 3. Drop public.idx_swipe_records_user_id.
--    swipe_records is appended on every HandleSwipe inside the user-facing swipe
--    transaction. The single-column (user_id) index is fully covered by the two
--    composites whose leftmost column is user_id:
--      idx_swipe_records_user_reviewed  (user_id, reviewed_at DESC)
--      idx_swipe_records_user_cardgroup (user_id, cardgroup_id, reviewed_at DESC)
--    Every access path is served by a composite: the PK (FindByIDs), the
--    user_cardgroup composite (FindByUserAndCardgroup), the user_reviewed
--    composite (ListRecentByUser), and the ON DELETE CASCADE FK / RLS
--    user_id-equality policies (either composite's leading column). The
--    single-column index is redundant write overhead on the hottest write table.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_users_created_at_id
    ON public.users (created_at DESC, id ASC);

DROP INDEX IF EXISTS public.idx_users_updated_at;

DROP INDEX IF EXISTS public.idx_swipe_records_user_id;

COMMIT;
