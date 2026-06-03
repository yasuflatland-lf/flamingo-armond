-- Reverse of 20260603090000_enable_rls_schema_migrations.up.sql.
--
-- Restores the pre-migration state: re-grant the Supabase-default API-role
-- privileges, then disable RLS. Running this down re-opens the PostgREST
-- exposure; it exists for migration symmetry and is not expected to run in
-- production.
--
-- Like the up migration, every statement is guarded on the existence of
-- public.schema_migrations via to_regclass, so the file is a no-op in harnesses
-- that apply migrations with psql instead of golang-migrate (where the table is
-- absent). golang-migrate keeps schema_migrations present even at version 0, so
-- the Go down/up roundtrip exercises the real branch.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NULL THEN
        RAISE NOTICE 'schema_migrations absent (non-golang-migrate harness); nothing to revert';
        RETURN;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'GRANT ALL ON public.schema_migrations TO anon';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'GRANT ALL ON public.schema_migrations TO authenticated';
    END IF;

    EXECUTE 'ALTER TABLE public.schema_migrations DISABLE ROW LEVEL SECURITY';
END
$$;

COMMIT;
