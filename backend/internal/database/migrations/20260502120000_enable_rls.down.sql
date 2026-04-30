-- Reverse of 20260502120000_enable_rls.up.sql.
--
-- Exists for golang-migrate symmetry so the migration runner can step down
-- cleanly in development and CI. Do NOT run in production unless deliberately
-- removing RLS from all public tables -- disabling RLS exposes all rows to
-- PostgREST anonymous callers.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.schema_migrations DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.ping_records      DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.swipe_records     DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.cards             DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.cardgroups        DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles        DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.roles             DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.users             DISABLE ROW LEVEL SECURITY;

COMMIT;
