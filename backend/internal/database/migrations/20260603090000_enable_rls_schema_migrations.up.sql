-- Enable Row Level Security on golang-migrate's bookkeeping table.
--
-- public.schema_migrations is created and owned by golang-migrate (which connects
-- as the postgres table owner). It lives in the public schema, which Supabase
-- exposes through PostgREST. Without RLS, the Supabase-default GRANTs on public
-- tables let the anon and authenticated roles read AND write this table over the
-- REST API (/rest/v1/schema_migrations) — an integrity risk: a caller could
-- corrupt the recorded version or set dirty=true and disrupt deploys. The
-- "owner bypasses RLS" property protects only the backend's and golang-migrate's
-- own owner-role connections; it says nothing about the PostgREST anon /
-- authenticated path, which is the real exposure the Supabase advisor flags.
--
-- The fix is deny-all RLS (ENABLE with no policy) plus an explicit REVOKE of the
-- API-role GRANTs. FORCE ROW LEVEL SECURITY is deliberately NOT set: the table
-- owner (golang-migrate, and the backend) keeps bypassing RLS, so version
-- bookkeeping and future deploys are unaffected. The resulting "RLS enabled, no
-- policy" state is the intended deny-all posture for an internal table; it
-- surfaces as a benign rls_enabled_no_policy INFO in the Supabase advisor rather
-- than the rls_disabled_in_public ERROR the missing RLS previously raised.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.schema_migrations ENABLE ROW LEVEL SECURITY;

-- Defense in depth: remove the PostgREST grant surface entirely. Guarded with
-- pg_roles probes so the migration stays portable to plain PostgreSQL
-- (testcontainers) where anon / authenticated may not exist.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON public.schema_migrations FROM anon;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON public.schema_migrations FROM authenticated;
    END IF;
END
$$;

COMMIT;
