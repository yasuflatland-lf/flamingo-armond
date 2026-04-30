-- Add ping_records table for service readiness monitoring.
--
-- Stores periodic ping heartbeat records with minimal data (just id, created_at,
-- updated_at). Table holds 0 or 1 row. Row Level Security is enabled with zero
-- policies so PostgREST callers default-deny; the backend connects as the
-- table-owner role and bypasses RLS.

-- ---------------------------------------------------------------------------
-- ping_records
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.ping_records (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Row Level Security: enable with zero policies.
-- ---------------------------------------------------------------------------

ALTER TABLE public.ping_records ENABLE ROW LEVEL SECURITY;
