-- Reverse of 20260603090000_enable_rls_schema_migrations.up.sql.
--
-- Restores the pre-migration state: re-grant the Supabase-default API-role
-- privileges, then disable RLS. Running this down re-opens the PostgREST
-- exposure; it exists for migration symmetry and is not expected to run in
-- production.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        GRANT ALL ON public.schema_migrations TO anon;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT ALL ON public.schema_migrations TO authenticated;
    END IF;
END
$$;

ALTER TABLE IF EXISTS public.schema_migrations DISABLE ROW LEVEL SECURITY;

COMMIT;
