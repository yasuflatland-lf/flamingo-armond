-- Seed the system "general" role alongside the pre-existing "admin" role.
--
-- Mirrors the admin seed in 20260430080000_initial_schema.up.sql so the
-- general role is provisioned idempotently on every fresh database. The
-- name "general" is the default end-user role; like "admin" it is treated
-- as a system role by the AdminRole usecase (rename/delete are blocked).
--
-- ON CONFLICT DO NOTHING keeps the up-migration idempotent across re-runs
-- and across environments where the row may already exist by accident.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

INSERT INTO public.roles (name) VALUES ('general')
    ON CONFLICT DO NOTHING;

COMMIT;
