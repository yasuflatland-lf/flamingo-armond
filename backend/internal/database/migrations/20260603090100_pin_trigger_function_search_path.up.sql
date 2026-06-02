-- Pin search_path on the updated_at trigger functions.
--
-- The four set_*_updated_at trigger functions were created without a fixed
-- search_path, which the Supabase advisor flags (function_search_path_mutable):
-- a role-mutable search_path lets a caller influence unqualified name resolution
-- inside the function. These functions only assign now() to NEW.updated_at and
-- reference no schema-qualified objects, so an empty search_path is both safe and
-- maximally strict (now() resolves from pg_catalog, which is always on the path).
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER FUNCTION public.set_users_updated_at()          SET search_path = '';
ALTER FUNCTION public.set_cardgroups_updated_at()     SET search_path = '';
ALTER FUNCTION public.set_cards_updated_at()          SET search_path = '';
ALTER FUNCTION public.set_user_card_fsrs_updated_at() SET search_path = '';

COMMIT;
