-- Auth-schema bootstrap for the SchemaSpy ephemeral Postgres instance.
--
-- WHY THIS FILE EXISTS
-- The consolidated initial migration was written for Supabase and therefore
-- references objects that Supabase manages but that do not exist on a plain
-- Postgres server:
--   * A foreign key to auth.users
--   * A trigger on auth.users
--   * RLS policies that call auth.uid()
-- Without this bootstrap applied first, the migration fails immediately on a
-- vanilla Postgres container with "schema auth does not exist".
--
-- APPLIED AS (in CI, against the ephemeral "armond" database):
--   psql -h localhost -U postgres -d armond -v ON_ERROR_STOP=1 -f tools/schemaspy/auth-stub.sql
-- before the migration up-files are run in the SchemaSpy CI workflow.
-- Adapt the -h / -U / -d flags when applying against a different local Postgres.
--
-- RELATIONSHIP TO INTEGRATION TESTS
-- This SQL block is identical to the bootstrap executed by the four backend
-- integration test suites before they run migrations:
--   internal/database/pool_test.go
--   internal/repository/user_test.go
--   cmd/server/main_test.go
--   cmd/seed/main_test.go
-- Deduplicating those four in-test copies into this file is intentionally
-- out of scope here; they remain independent.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        CREATE ROLE authenticated LOGIN PASSWORD 'test';
    END IF;
END
$$;
CREATE SCHEMA IF NOT EXISTS auth;
CREATE TABLE IF NOT EXISTS auth.users (
    id uuid PRIMARY KEY,
    email text
);
CREATE OR REPLACE FUNCTION auth.uid()
RETURNS uuid
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(
        NULLIF(current_setting('request.jwt.claim.sub', true), ''),
        NULLIF(current_setting('request.jwt.claims', true), '')::jsonb ->> 'sub'
    )::uuid
$$;
GRANT USAGE ON SCHEMA auth TO authenticated;
GRANT EXECUTE ON FUNCTION auth.uid() TO authenticated;
GRANT USAGE ON SCHEMA public TO authenticated;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO authenticated;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO authenticated;
