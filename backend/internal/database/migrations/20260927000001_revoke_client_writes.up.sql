-- Direct API writes bypass Go-side rules; RLS remains as defence in depth.
-- SELECT stays granted, but TRUNCATE cannot rely on RLS for protection.
-- Default privileges cover future tables created by the migration role.
-- Plain PostgreSQL harnesses may lack the API roles; probe before revoking.
-- pgx/v5 does not auto-wrap migrations, so use an explicit transaction.

BEGIN;

DO $$
DECLARE
    api_role text;
BEGIN
    FOREACH api_role IN ARRAY ARRAY['anon', 'authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = api_role) THEN
            -- schema_migrations is not listed: 20260603090000 already revoked ALL, and psql-only harnesses (ER chart) never create it.
            EXECUTE format(
                'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE '
                || 'public.users, public.roles, public.user_roles, '
                || 'public.cardgroups, public.cards, public.swipe_records, '
                || 'public.ping_records, public.user_card_fsrs, '
                || 'public.user_preferences, public.master_cardgroups, '
                || 'public.master_cards FROM %I',
                api_role);
            EXECUTE format(
                'ALTER DEFAULT PRIVILEGES IN SCHEMA public '
                || 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLES FROM %I',
                api_role);
        END IF;
    END LOOP;
END
$$;

COMMIT;
