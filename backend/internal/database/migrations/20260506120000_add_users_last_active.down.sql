-- Reverse 20260506120000_add_users_last_active.up.sql.
-- Reverse the up migration in opposite creation order: drop the explicit
-- index first, then the column. (Postgres would also auto-drop the index
-- when the column is dropped, but stating it explicitly keeps the audit
-- trail symmetric with the up migration.)

BEGIN;

DROP INDEX IF EXISTS public.idx_users_last_active;

ALTER TABLE public.users
    DROP COLUMN IF EXISTS last_active;

COMMIT;
