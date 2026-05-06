-- Reverse 20260506120000_add_users_last_active.up.sql.
-- Drop the index before the column so the index does not become orphaned
-- mid-migration if a step fails.

BEGIN;

DROP INDEX IF EXISTS public.idx_users_last_active;

ALTER TABLE public.users
    DROP COLUMN IF EXISTS last_active;

COMMIT;
