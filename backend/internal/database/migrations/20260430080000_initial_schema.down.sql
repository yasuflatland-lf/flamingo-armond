-- Reverse of 20260430080000_initial_schema.up.sql.
-- Drops tables, functions, and triggers in reverse dependency order so foreign
-- keys do not block the teardown. RLS is managed by the separate
-- 20260502120000_enable_rls migration; no DISABLE ROW LEVEL SECURITY is needed
-- here because DROP TABLE removes the table and its RLS state entirely.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP TABLE IF EXISTS public.swipe_records;

DROP TRIGGER  IF EXISTS trg_cards_set_updated_at ON public.cards;
DROP FUNCTION IF EXISTS public.set_cards_updated_at();
DROP TABLE    IF EXISTS public.cards;

DROP TRIGGER  IF EXISTS trg_cardgroups_set_updated_at ON public.cardgroups;
DROP FUNCTION IF EXISTS public.set_cardgroups_updated_at();
DROP TABLE    IF EXISTS public.cardgroups;

DROP TABLE IF EXISTS public.user_roles;
DROP TABLE IF EXISTS public.roles;

DROP TRIGGER  IF EXISTS trg_handle_new_user ON auth.users;
DROP FUNCTION IF EXISTS public.handle_new_user();
DROP TRIGGER  IF EXISTS trg_users_set_updated_at ON public.users;
DROP FUNCTION IF EXISTS public.set_users_updated_at();
DROP TABLE    IF EXISTS public.users;

COMMIT;
