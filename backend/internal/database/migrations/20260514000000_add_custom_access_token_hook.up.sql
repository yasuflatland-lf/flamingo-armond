-- Add the Supabase Custom Access Token Hook used by GoTrue at JWT mint time.
--
-- The hook joins public.user_roles + public.roles whenever a new access token is
-- issued and writes app_metadata.role = "admin" into the JWT claims when the
-- user has the admin role. Frontends read this via supabase.auth.getClaims()
-- and skip the round-trip to the GraphQL backend for header role gating.
--
-- Why join-at-mint instead of a sync trigger that mirrors user_roles into
-- auth.users.raw_app_meta_data: the single source of truth for role membership
-- stays in public.user_roles. A trigger-based mirror would duplicate state and
-- silently drift if the trigger is ever skipped (manual SQL, restore from
-- backup, schema change). The mint-time hook recomputes from the source on
-- every token rotation, so revocations propagate at the next refresh.
--
-- Contract (Supabase Custom Access Token Hook):
--   event jsonb has shape { "user_id": uuid, "claims": jsonb, ... }.
--   The function MUST return the modified event with the "claims" object
--   updated in place — every other claim the auth service prepared must be
--   preserved.
--
-- STABLE: reads tables, must not be IMMUTABLE.
-- SECURITY DEFINER: function executes as owner so supabase_auth_admin can
-- invoke it without needing direct SELECT on public.user_roles / public.roles.
-- search_path locked to public to neutralise SECURITY DEFINER injection vector.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE OR REPLACE FUNCTION public.custom_access_token_hook(event jsonb)
RETURNS jsonb
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    uid uuid;
    original_claims jsonb;
    app_metadata jsonb;
    is_admin_user boolean;
    new_claims jsonb;
BEGIN
    uid := (event->>'user_id')::uuid;
    original_claims := event->'claims';
    app_metadata := COALESCE(original_claims->'app_metadata', '{}'::jsonb);

    SELECT EXISTS (
        SELECT 1
        FROM public.user_roles ur
        JOIN public.roles r ON r.id = ur.role_id
        WHERE ur.user_id = uid
          AND r.name = 'admin'
    ) INTO is_admin_user;

    IF is_admin_user THEN
        app_metadata := jsonb_set(app_metadata, '{role}', '"admin"'::jsonb, true);
    ELSE
        -- Clear any stale role claim so a revoked admin does not keep a stale
        -- token-side metadata entry on refresh.
        app_metadata := app_metadata - 'role';
    END IF;

    new_claims := jsonb_set(original_claims, '{app_metadata}', app_metadata, true);
    RETURN jsonb_set(event, '{claims}', new_claims, true);
END;
$$;

-- Lock down access: only the Supabase auth role that mints tokens may call
-- this function. PostgREST callers (authenticated, anon) must never invoke it.
REVOKE ALL ON FUNCTION public.custom_access_token_hook(jsonb) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON FUNCTION public.custom_access_token_hook(jsonb) FROM authenticated;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON FUNCTION public.custom_access_token_hook(jsonb) FROM anon;
    END IF;
END
$$;

-- Grant execute to the Supabase-managed role that mints tokens. The test
-- container is a plain PostgreSQL instance without this role, so the grant is
-- skipped there to keep migrations idempotent across both environments.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN
        GRANT EXECUTE ON FUNCTION public.custom_access_token_hook(jsonb) TO supabase_auth_admin;
    END IF;
END
$$;

COMMIT;
