-- Reverse of 20260430080000_initial_schema.up.sql.
-- Disable RLS first, then drop tables / functions / triggers in reverse
-- dependency order so foreign keys do not block the teardown.

ALTER TABLE IF EXISTS public.schema_migrations DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.swipe_records     DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.cards             DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.cardgroups        DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.user_roles        DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.roles             DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.users             DISABLE ROW LEVEL SECURITY;

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
