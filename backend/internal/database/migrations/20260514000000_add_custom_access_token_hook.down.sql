-- Drops the custom access token hook function.
--
-- DROP FUNCTION automatically revokes all privileges granted on the function
-- object, so the REVOKE/GRANT blocks from the up migration do not need
-- explicit reversal here.
--
-- Operator precondition: before applying this migration, disable the hook in
-- supabase/config.toml ([auth.hook.custom_access_token] enabled = false) and
-- redeploy the auth service. Dropping the function while the hook is still
-- enabled will fail every subsequent token mint.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP FUNCTION IF EXISTS public.custom_access_token_hook(jsonb);

COMMIT;
