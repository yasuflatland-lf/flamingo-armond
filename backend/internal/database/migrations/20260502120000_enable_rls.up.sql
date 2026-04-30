-- Enable Row Level Security on every public table in a single atomic migration.
--
-- Rationale for isolation in its own migration file:
--   golang-migrate's pgx/v5 driver does NOT auto-wrap each migration file in
--   a transaction. Mixing DDL with ENABLE ROW LEVEL SECURITY in a single
--   migration file risks partial commit: some statements succeed, others fail,
--   and schema_migrations.dirty is set to true. Isolating RLS here limits the
--   blast radius -- if this migration fails, only the RLS step is dirty; the
--   tables themselves remain intact.
--
-- Transaction: explicit BEGIN/COMMIT is required because golang-migrate pgx/v5
--   does NOT wrap migration files in a transaction automatically.
--
-- Idempotency: ALTER TABLE ... ENABLE ROW LEVEL SECURITY is a no-op if RLS is
--   already enabled on the table, so re-running this migration or applying it
--   after a manual recovery step is safe.
--
-- Tables covered (public schema, after migrations 20260430080000,
--   20260501000000, and 20260502000000):
--   users, roles, user_roles, cardgroups, cards, swipe_records,
--   ping_records.

BEGIN;

ALTER TABLE public.users             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.roles             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cardgroups        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cards             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.swipe_records     ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.ping_records      ENABLE ROW LEVEL SECURITY;

COMMIT;
