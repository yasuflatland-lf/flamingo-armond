-- Reverse of 20260514000000_add_custom_access_token_hook.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP FUNCTION IF EXISTS public.custom_access_token_hook(jsonb);

COMMIT;
