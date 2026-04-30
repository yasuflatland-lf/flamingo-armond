-- Add the RBAC helper function consumed by upcoming RLS policies and Go handlers.
-- STABLE: reads tables, must not be IMMUTABLE.
-- SECURITY DEFINER: function executes as owner so RLS-enabled callers can probe.
-- search_path locked to public to neutralise SECURITY DEFINER injection vector.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE OR REPLACE FUNCTION public.is_admin(uid uuid)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM public.user_roles ur
        JOIN public.roles r ON r.id = ur.role_id
        WHERE ur.user_id = uid
          AND r.name = 'admin'
    );
$$;

-- Lock down access: anonymous PostgREST callers must not probe role membership.
REVOKE ALL ON FUNCTION public.is_admin(uuid) FROM PUBLIC;
-- Grant only when the Supabase-managed "authenticated" role exists (production).
-- The test container is a plain PostgreSQL instance without this role, so the
-- grant is skipped there to keep migrations idempotent across both environments.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT EXECUTE ON FUNCTION public.is_admin(uuid) TO authenticated;
    END IF;
END
$$;

COMMIT;
