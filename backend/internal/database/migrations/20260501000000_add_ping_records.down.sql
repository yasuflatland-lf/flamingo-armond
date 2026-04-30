-- Reverse of 20260501000000_add_ping_records.up.sql.
-- Disable RLS, then drop the ping_records table.

ALTER TABLE IF EXISTS public.ping_records DISABLE ROW LEVEL SECURITY;

DROP TABLE IF EXISTS public.ping_records;
