-- Add a unique index on public.cards (cardgroup_id, front) to support
-- the upsertDictionary pipeline's ON CONFLICT (cardgroup_id, front) clause.
--
-- Pre-check: enumerate every offending (cardgroup_id, front) pair via RAISE NOTICE so
-- operators can resolve them in a single pass. CREATE UNIQUE INDEX alone would only
-- name one offending pair on first failure. Both this RAISE EXCEPTION and the index
-- failure leave golang-migrate in a dirty state — recovery via Force(predecessor) is
-- standard.
--
-- Column ordering (cardgroup_id, front) matches the upsert's WHERE
-- prefix: cardgroup_id is the high-cardinality leading key, so the index
-- is also useful for per-group lookups, while still enforcing per-group
-- "front" uniqueness.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction;
-- the explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- Step 1: pre-check for existing duplicates and abort with a clear error.
DO $$
DECLARE
    dup_count integer;
    dup_row   record;
BEGIN
    SELECT COUNT(*) INTO dup_count
    FROM (
        SELECT cardgroup_id, front
        FROM public.cards
        GROUP BY cardgroup_id, front
        HAVING COUNT(*) > 1
    ) AS dups;

    IF dup_count > 0 THEN
        FOR dup_row IN
            SELECT cardgroup_id, front, COUNT(*) AS n
            FROM public.cards
            GROUP BY cardgroup_id, front
            HAVING COUNT(*) > 1
            ORDER BY cardgroup_id, front
        LOOP
            RAISE NOTICE 'duplicate (cardgroup_id, front): cardgroup_id=% front=% count=%',
                dup_row.cardgroup_id, dup_row.front, dup_row.n;
        END LOOP;
        RAISE EXCEPTION
            'cards table has duplicate (cardgroup_id, front) rows: % rows; resolve before adding the unique index',
            dup_count;
    END IF;
END
$$;

-- Step 2: create the unique index. IF NOT EXISTS keeps the migration safe
-- to re-run on a partially-applied database.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cards_cardgroup_front
    ON public.cards (cardgroup_id, front);

COMMIT;
