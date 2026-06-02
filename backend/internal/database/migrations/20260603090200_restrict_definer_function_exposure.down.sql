-- Reverse of 20260603090200_restrict_definer_function_exposure.up.sql.
--
-- Moves is_admin back into the public schema (so the initial-schema down can drop
-- it by its original name), drops the now-empty private schema, and restores the
-- Supabase-default EXECUTE grants. Running this down re-exposes the functions; it
-- exists for migration symmetry and is not expected to run in production.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- Move is_admin back to public (reverse of the SET SCHEMA above), then drop the
-- now-empty private schema.
ALTER FUNCTION private.is_admin(uuid) SET SCHEMA public;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE USAGE ON SCHEMA private FROM authenticated;
    END IF;
END
$$;
DROP SCHEMA IF EXISTS private;

-- Restore the API-role grants revoked in Up.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        GRANT EXECUTE ON FUNCTION public.is_admin(uuid) TO anon;
    END IF;
END
$$;

GRANT EXECUTE ON FUNCTION public.handle_new_user() TO PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT EXECUTE ON FUNCTION public.handle_new_user() TO authenticated;
    END IF;
END
$$;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        GRANT EXECUTE ON FUNCTION public.handle_new_user() TO anon;
    END IF;
END
$$;

COMMIT;
