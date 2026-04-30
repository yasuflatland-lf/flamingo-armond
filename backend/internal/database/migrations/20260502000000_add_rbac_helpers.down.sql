-- Reverse of 20260502000000_add_rbac_helpers.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP FUNCTION IF EXISTS public.is_admin(uuid);

COMMIT;
