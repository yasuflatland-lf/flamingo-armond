-- Reverse of 20260521090000_add_version_to_users.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.users
    DROP COLUMN IF EXISTS version;

COMMIT;
