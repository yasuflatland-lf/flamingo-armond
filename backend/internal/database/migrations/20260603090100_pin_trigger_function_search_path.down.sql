-- Reverse of 20260603090100_pin_trigger_function_search_path.up.sql.
--
-- RESET search_path returns each function to the role-mutable default it had
-- before the up migration.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER FUNCTION public.set_users_updated_at()          RESET search_path;
ALTER FUNCTION public.set_cardgroups_updated_at()     RESET search_path;
ALTER FUNCTION public.set_cards_updated_at()          RESET search_path;
ALTER FUNCTION public.set_user_card_fsrs_updated_at() RESET search_path;

COMMIT;
