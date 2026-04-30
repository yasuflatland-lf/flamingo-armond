-- Add ping_records table for service readiness monitoring.
--
-- Stores periodic ping heartbeat records with minimal data (id, created_at,
-- updated_at). Row Level Security for this table is enabled in migration
-- 20260502120000_enable_rls, which covers all public tables in a single atomic
-- transaction.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE TABLE IF NOT EXISTS public.ping_records (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

COMMIT;
