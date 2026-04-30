-- Reverse of 20260501000000_add_ping_records.up.sql.
-- RLS on this table is managed by the separate 20260502120000_enable_rls
-- migration; DROP TABLE removes RLS state with the table.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP TABLE IF EXISTS public.ping_records;

COMMIT;
