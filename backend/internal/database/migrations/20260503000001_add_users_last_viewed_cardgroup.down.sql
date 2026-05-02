-- Reverse 20260503000001_add_users_last_viewed_cardgroup.up.sql.
-- Drop the index before the column so the index does not become orphaned
-- mid-migration if a step fails.

BEGIN;

DROP INDEX IF EXISTS public.idx_users_last_viewed_cardgroup_id;

ALTER TABLE public.users
    DROP COLUMN IF EXISTS last_viewed_cardgroup_id;

COMMIT;
