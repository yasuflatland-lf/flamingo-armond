-- Restrict PostgREST exposure of the SECURITY DEFINER helper functions.
--
-- Two functions were callable as PostgREST RPCs by anon / authenticated, which
-- the Supabase advisor flags (anon/authenticated_security_definer_function_executable):
--
--   * handle_new_user() is a trigger function on auth.users. Triggers fire with
--     the table owner's privileges, so no API role needs EXECUTE on it. Revoking
--     the inherited grants removes it from /rest/v1/rpc/handle_new_user.
--
--   * is_admin(uuid) is a helper that RLS policies invoke. The authenticated role
--     MUST keep EXECUTE for policy evaluation, so it cannot simply be revoked.
--     Instead, move it into a dedicated `private` schema that PostgREST does not
--     expose. ALTER FUNCTION ... SET SCHEMA preserves the function OID, so the
--     RLS policies that reference it keep resolving unchanged; only the
--     API-surface name (/rest/v1/rpc/is_admin) disappears. anon never needs
--     is_admin and is revoked outright (the initial migration intended this but
--     only revoked PUBLIC, leaving the Supabase-default anon grant in place).
--
-- Guards use pg_roles probes for parity with the rest of the schema and to stay
-- portable to plain PostgreSQL (testcontainers).
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- handle_new_user: trigger-only; no API role needs EXECUTE.
REVOKE ALL ON FUNCTION public.handle_new_user() FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON FUNCTION public.handle_new_user() FROM authenticated;
    END IF;
END
$$;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON FUNCTION public.handle_new_user() FROM anon;
    END IF;
END
$$;

-- is_admin: anon must not reach it (RLS uses it only for the authenticated role).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON FUNCTION public.is_admin(uuid) FROM anon;
    END IF;
END
$$;

-- Move is_admin out of the PostgREST-exposed public schema. The OID is preserved,
-- so RLS policies that reference it keep resolving and the authenticated EXECUTE
-- grant travels with the function; authenticated still needs USAGE on the new
-- schema to reach it.
CREATE SCHEMA IF NOT EXISTS private;
ALTER FUNCTION public.is_admin(uuid) SET SCHEMA private;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT USAGE ON SCHEMA private TO authenticated;
    END IF;
END
$$;

COMMIT;
