-- Add public.users.last_active to back the admin DataTable's "Last active" column.
-- Populated by backend/internal/auth/middleware.go on every authenticated request.
-- Nullable so existing rows do not need a backfill; null renders as "Never" in the UI.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS last_active TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_users_last_active
    ON public.users (last_active DESC NULLS LAST);

COMMIT;
