-- Restoring API writes re-opens the path that bypasses Go-side rules.
-- This reversal exists for migration symmetry, not production use.
-- Plain PostgreSQL harnesses may lack the API roles; probe before granting.
-- pgx/v5 does not auto-wrap migrations, so use an explicit transaction.

BEGIN;

DO $$
DECLARE
    api_role text;
BEGIN
    FOREACH api_role IN ARRAY ARRAY['anon', 'authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = api_role) THEN
            EXECUTE format(
                'GRANT INSERT, UPDATE, DELETE, TRUNCATE ON TABLE '
                || 'public.users, public.roles, public.user_roles, '
                || 'public.cardgroups, public.cards, public.swipe_records, '
                || 'public.ping_records, public.user_card_fsrs, '
                || 'public.user_preferences, public.master_cardgroups, '
                || 'public.master_cards TO %I',
                api_role);
            EXECUTE format(
                'ALTER DEFAULT PRIVILEGES IN SCHEMA public '
                || 'GRANT INSERT, UPDATE, DELETE, TRUNCATE ON TABLES TO %I',
                api_role);
        END IF;
    END LOOP;
END
$$;

COMMIT;
