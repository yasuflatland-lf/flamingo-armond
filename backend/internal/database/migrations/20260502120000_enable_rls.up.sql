-- Enable Row Level Security on every public table in a single atomic migration.
--
-- Rationale for isolation in its own migration file:
--   The prior incident (2026-04-30) occurred because golang-migrate's pgx/v5
--   driver does NOT auto-wrap each migration file in a transaction. When the
--   initial schema migration contained both DDL and ENABLE ROW LEVEL SECURITY
--   statements, a mid-file failure left the database with tables created but
--   RLS not enabled, while schema_migrations.dirty was set to true. Isolating
--   RLS in its own migration limits the blast radius: if this migration fails,
--   only the RLS step is dirty -- the tables themselves remain intact.
--
-- Transaction: explicit BEGIN/COMMIT is required because golang-migrate pgx/v5
--   does NOT wrap migration files in a transaction automatically.
--
-- Idempotency: ALTER TABLE ... ENABLE ROW LEVEL SECURITY is a no-op if RLS is
--   already enabled on the table. Production has had RLS manually re-applied
--   after the 2026-04-30 incident, so this migration runs as a safe no-op
--   against that environment.
--
-- Tables covered (public schema, after migrations 20260430080000,
--   20260501000000, and 20260502000000):
--   users, roles, user_roles, cardgroups, cards, swipe_records,
--   ping_records, schema_migrations.

BEGIN;

ALTER TABLE public.users             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.roles             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cardgroups        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cards             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.swipe_records     ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.ping_records      ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.schema_migrations ENABLE ROW LEVEL SECURITY;

COMMIT;
