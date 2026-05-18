-- Reverse of 20260430080000_initial_schema.up.sql.
--
-- Dependency-reverse order: strip the trigger on auth.users first so the
-- Supabase-managed table is left clean, then drop policies, RLS state, public
-- triggers, tables, and finally the functions the triggers and policies
-- referenced. Every DROP is IF EXISTS; no CASCADE — explicit ordering is
-- safer than letting CASCADE silently pull in objects added by future code.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) Trigger on auth.users — strip first.
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_handle_new_user ON auth.users;

-- ---------------------------------------------------------------------------
-- 2) Revoke privileges granted in Up so DROP FUNCTION later runs against an
--    ungranted function. Guarded with pg_roles probes for parity with Up.
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN
        REVOKE EXECUTE ON FUNCTION public.custom_access_token_hook(jsonb) FROM supabase_auth_admin;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE EXECUTE ON FUNCTION public.is_admin(uuid) FROM authenticated;
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 3) Policies (reverse of Up).
-- ---------------------------------------------------------------------------

DROP POLICY IF EXISTS user_preferences_delete_own_or_admin ON public.user_preferences;
DROP POLICY IF EXISTS user_preferences_update_own_or_admin ON public.user_preferences;
DROP POLICY IF EXISTS user_preferences_insert_own_or_admin ON public.user_preferences;
DROP POLICY IF EXISTS user_preferences_select_own_or_admin ON public.user_preferences;

DROP POLICY IF EXISTS user_card_fsrs_self ON public.user_card_fsrs;

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

-- ---------------------------------------------------------------------------
-- 4) Disable RLS (formal symmetry with Up; DROP TABLE below would also clear
--    the flag, but the explicit DISABLE keeps the rollback observable on a
--    partially-applied database).
-- ---------------------------------------------------------------------------

ALTER TABLE IF EXISTS public.user_preferences  DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.user_card_fsrs    DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.ping_records      DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.swipe_records     DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.cards             DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.cardgroups        DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.user_roles        DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.roles             DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.users             DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 5) Triggers on public.* (the auth.users trigger was dropped at step 1).
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_user_card_fsrs_set_updated_at ON public.user_card_fsrs;
DROP TRIGGER IF EXISTS trg_cards_set_updated_at         ON public.cards;
DROP TRIGGER IF EXISTS trg_cardgroups_set_updated_at    ON public.cardgroups;
DROP TRIGGER IF EXISTS trg_users_set_updated_at         ON public.users;

-- ---------------------------------------------------------------------------
-- 6) Explicit index drop for indexes that should fail loudly if missing
--    (the unique index is the contract that backs the upsertDictionary
--    ON CONFLICT clause). Plain idx_* drops are implied by DROP TABLE.
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS public.uq_cards_cardgroup_front;

-- ---------------------------------------------------------------------------
-- 7) Tables (FK-referenced ones last).
-- ---------------------------------------------------------------------------

DROP TABLE IF EXISTS public.user_preferences;
DROP TABLE IF EXISTS public.user_card_fsrs;
DROP TABLE IF EXISTS public.ping_records;
DROP TABLE IF EXISTS public.swipe_records;
DROP TABLE IF EXISTS public.cards;
DROP TABLE IF EXISTS public.cardgroups;
DROP TABLE IF EXISTS public.user_roles;
DROP TABLE IF EXISTS public.users;
DROP TABLE IF EXISTS public.roles;

-- ---------------------------------------------------------------------------
-- 8) Functions (after every dependent object is gone).
-- ---------------------------------------------------------------------------

DROP FUNCTION IF EXISTS public.custom_access_token_hook(jsonb);
DROP FUNCTION IF EXISTS public.is_admin(uuid);
DROP FUNCTION IF EXISTS public.handle_new_user();
DROP FUNCTION IF EXISTS public.set_user_card_fsrs_updated_at();
DROP FUNCTION IF EXISTS public.set_cards_updated_at();
DROP FUNCTION IF EXISTS public.set_cardgroups_updated_at();
DROP FUNCTION IF EXISTS public.set_users_updated_at();

COMMIT;
