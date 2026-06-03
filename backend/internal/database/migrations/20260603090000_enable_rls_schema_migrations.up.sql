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
-- public.schema_migrations is created by golang-migrate itself, NOT by any
-- migration SQL file. Harnesses that apply these up files directly with psql
-- (e.g. the SchemaSpy ER-chart workflow in .github/workflows/er-chart.yml) never
-- run golang-migrate, so the table is absent there. Every statement below is
-- therefore guarded on the table's existence via to_regclass, making this
-- migration a no-op in that case while still enabling deny-all RLS under
-- golang-migrate (production, and the Go testcontainer suite). The guard also
-- requires dynamic EXECUTE because ALTER TABLE / REVOKE cannot be made
-- conditional otherwise.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NULL THEN
        RAISE NOTICE 'schema_migrations absent (non-golang-migrate harness); skipping RLS hardening';
        RETURN;
    END IF;

    EXECUTE 'ALTER TABLE public.schema_migrations ENABLE ROW LEVEL SECURITY';

    -- Defense in depth: remove the PostgREST grant surface entirely. Guarded
    -- with pg_roles probes so the migration stays portable to plain PostgreSQL
    -- (testcontainers) where anon / authenticated may not exist.
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL ON public.schema_migrations FROM anon';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL ON public.schema_migrations FROM authenticated';
    END IF;
END
$$;

COMMIT;
