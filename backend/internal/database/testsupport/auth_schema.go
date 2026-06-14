// Package testsupport provides shared helpers for integration tests that run
// against a real Postgres (via testcontainers). It deliberately holds no test
// files itself so it can be imported as a normal package from any *_test.go in
// the backend.
package testsupport

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rotisserie/eris"
)

// BootstrapAuthSchema mimics the Supabase-managed auth schema and roles just
// enough for FK, trigger, and RLS policy references in migrations to resolve.
// It connects to dsn, creates the `authenticated` role, the `auth` schema with
// the `auth.users` table and `auth.uid()` function, and grants the privileges
// (including ALTER DEFAULT PRIVILEGES for future tables/sequences) that RLS
// policies rely on. Run it before database.Migrate against a throwaway test DB.
func BootstrapAuthSchema(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return eris.Wrap(err, "testsupport: parse dsn")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return eris.Wrap(err, "testsupport: open pool")
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
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
    `); err != nil {
		return eris.Wrap(err, "testsupport: exec bootstrap auth schema")
	}
	return nil
}
