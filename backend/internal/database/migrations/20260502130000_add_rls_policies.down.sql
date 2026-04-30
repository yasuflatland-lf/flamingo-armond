-- Drop the RLS policies added in 20260502130000_add_rls_policies.up.sql.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP POLICY IF EXISTS user_roles_delete_admin ON public.user_roles;
DROP POLICY IF EXISTS user_roles_update_admin ON public.user_roles;
DROP POLICY IF EXISTS user_roles_insert_admin ON public.user_roles;
DROP POLICY IF EXISTS user_roles_select_self_or_admin ON public.user_roles;

DROP POLICY IF EXISTS roles_delete_admin ON public.roles;
DROP POLICY IF EXISTS roles_update_admin ON public.roles;
DROP POLICY IF EXISTS roles_insert_admin ON public.roles;
DROP POLICY IF EXISTS roles_select_public ON public.roles;

DROP POLICY IF EXISTS swipe_records_insert_own ON public.swipe_records;
DROP POLICY IF EXISTS swipe_records_select_own_or_admin ON public.swipe_records;

DROP POLICY IF EXISTS cards_all_owner_or_admin ON public.cards;
DROP POLICY IF EXISTS cardgroups_all_owner_or_admin ON public.cardgroups;
DROP POLICY IF EXISTS users_update_own_or_admin ON public.users;
DROP POLICY IF EXISTS users_select_own_or_admin ON public.users;

COMMIT;
