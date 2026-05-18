-- Consolidated initial schema for flamingo-armond.
--
-- Consolidates the prior 11 up/down migration pairs into a single initial-schema
-- pair; the resulting public schema is equivalent to applying those steps in order.
--
-- Order within this file:
--   1. Trigger functions and the handle_new_user bridge (plpgsql; late-bind, so
--      they may be created before the tables they reference at trigger-fire
--      time).
--   2. Tables in foreign-key order
--      (roles -> users -> user_roles -> cardgroups -> cards -> swipe_records
--       -> ping_records -> user_card_fsrs -> user_preferences), with per-table
--      indexes interleaved.
--   3. RBAC + JWT helper functions (is_admin, custom_access_token_hook).
--      is_admin is LANGUAGE sql, which is early-bind in PostgreSQL: the tables
--      it references must already exist at CREATE FUNCTION time.
--   4. Triggers (public.* first, then the trigger on auth.users).
--   5. RLS enable + policies (ENABLE ROW LEVEL SECURITY, then CREATE POLICY).
--   6. REVOKE / GRANT statements that gate the SECURITY DEFINER functions.
--   7. Seed inserts for the roles table (admin, general).
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) Trigger functions and the auth.users bridge (plpgsql; late-bind)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION public.set_users_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.set_cardgroups_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.set_cards_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.set_user_card_fsrs_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- Auto-create a public.users row when a new auth.users row is inserted.
CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    INSERT INTO public.users (id) VALUES (NEW.id)
        ON CONFLICT (id) DO NOTHING;
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- 2) Tables (FK order) + per-table indexes
-- ---------------------------------------------------------------------------

-- roles: referenced by user_roles.
CREATE TABLE IF NOT EXISTS public.roles (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE
);

-- users: 1:1 with auth.users. Populated by the handle_new_user trigger that
-- fires on auth.users insert.
CREATE TABLE IF NOT EXISTS public.users (
    id            uuid PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    display_name  text,
    bio           text,
    avatar_url    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_updated_at ON public.users (updated_at);

-- user_roles: many-to-many join between users and roles.
CREATE TABLE IF NOT EXISTS public.user_roles (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES public.roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON public.user_roles (role_id);

-- cardgroups: owned 1:N by users.
CREATE TABLE IF NOT EXISTS public.cardgroups (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid        NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name        text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cardgroups_name_length CHECK (char_length(trim(name)) BETWEEN 1 AND 100)
);

CREATE INDEX IF NOT EXISTS idx_cardgroups_owner_id   ON public.cardgroups (owner_id);
CREATE INDEX IF NOT EXISTS idx_cardgroups_updated_at ON public.cardgroups (updated_at);

-- cards: text content only. Per-user FSRS scheduling state lives in
-- public.user_card_fsrs, populated by review activity.
CREATE TABLE IF NOT EXISTS public.cards (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    cardgroup_id    uuid        NOT NULL REFERENCES public.cardgroups(id) ON DELETE CASCADE,
    front           text        NOT NULL,
    back            text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 500),
    CONSTRAINT cards_back_length  CHECK (char_length(trim(back))  BETWEEN 1 AND 500)
);

CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_id  ON public.cards (cardgroup_id);

-- Unique index supporting the upsertDictionary pipeline's
-- ON CONFLICT (cardgroup_id, front) clause.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cards_cardgroup_front
    ON public.cards (cardgroup_id, front);

-- swipe_records: immutable review log; no updated_at.
CREATE TABLE IF NOT EXISTS public.swipe_records (
    id             uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid             NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    card_id        uuid             NOT NULL REFERENCES public.cards(id) ON DELETE CASCADE,
    rating         integer          NOT NULL CHECK (rating BETWEEN 1 AND 4),
    reviewed_at    timestamptz      NOT NULL DEFAULT now(),

    due            timestamptz      NOT NULL,
    stability      double precision NOT NULL,
    difficulty     double precision NOT NULL,
    elapsed_days   integer          NOT NULL,
    scheduled_days integer          NOT NULL,
    reps           integer          NOT NULL,
    lapses         integer          NOT NULL,
    state          integer          NOT NULL,
    last_review    timestamptz      NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_swipe_records_user_id       ON public.swipe_records (user_id);
CREATE INDEX IF NOT EXISTS idx_swipe_records_card_id       ON public.swipe_records (card_id);
CREATE INDEX IF NOT EXISTS idx_swipe_records_user_reviewed ON public.swipe_records (user_id, reviewed_at DESC);

-- ping_records: service readiness heartbeat.
CREATE TABLE IF NOT EXISTS public.ping_records (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- user_card_fsrs: per-user FSRS scheduling state for each card.
CREATE TABLE IF NOT EXISTS public.user_card_fsrs (
    user_id        uuid             NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    card_id        uuid             NOT NULL REFERENCES public.cards(id) ON DELETE CASCADE,
    state          integer          NOT NULL,
    due            timestamptz      NOT NULL,
    stability      double precision NOT NULL,
    difficulty     double precision NOT NULL,
    reps           integer          NOT NULL,
    lapses         integer          NOT NULL,
    last_review    timestamptz      NOT NULL,
    elapsed_days   integer          NOT NULL,
    scheduled_days integer          NOT NULL,
    created_at     timestamptz      NOT NULL DEFAULT now(),
    updated_at     timestamptz      NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, card_id)
);

CREATE INDEX IF NOT EXISTS idx_user_card_fsrs_user_due
    ON public.user_card_fsrs (user_id, due);

-- user_preferences: UI continuity state (last_viewed_cardgroup_id, etc.) kept
-- out of users so the User repository does not need to depend on cardgroups.
CREATE TABLE IF NOT EXISTS public.user_preferences (
    user_id                  uuid PRIMARY KEY
        REFERENCES public.users(id) ON DELETE CASCADE,
    last_viewed_cardgroup_id uuid NULL
        REFERENCES public.cardgroups(id) ON DELETE SET NULL,
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_preferences_last_viewed_cardgroup_id
    ON public.user_preferences (last_viewed_cardgroup_id);

-- ---------------------------------------------------------------------------
-- 3) RBAC + JWT helper functions
-- ---------------------------------------------------------------------------

-- RBAC helper consumed by RLS policies and Go handlers.
-- STABLE: reads tables, must not be IMMUTABLE.
-- SECURITY DEFINER: function executes as owner so RLS-enabled callers can probe.
-- search_path locked to public to neutralise SECURITY DEFINER injection vector.
CREATE OR REPLACE FUNCTION public.is_admin(uid uuid)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM public.user_roles ur
        JOIN public.roles r ON r.id = ur.role_id
        WHERE ur.user_id = uid
          AND r.name = 'admin'
    );
$$;

-- Supabase Custom Access Token Hook used by GoTrue at JWT mint time.
-- Joins public.user_roles + public.roles and writes app_metadata.role = "admin"
-- into the JWT claims when the user has the admin role. Frontends read this via
-- supabase.auth.getClaims() and skip the round-trip to the GraphQL backend for
-- header role gating.
--
-- The function fails closed on a malformed event (missing user_id, or missing
-- or null claims) by raising an exception, so a corrupt payload cannot
-- silently mint a token with NULL user lookup or destroyed claims.
--
-- STABLE: reads tables, must not be IMMUTABLE.
-- SECURITY DEFINER: function executes as owner so supabase_auth_admin can
-- invoke it without needing direct SELECT on public.user_roles / public.roles.
-- search_path locked to public to neutralise SECURITY DEFINER injection vector.
CREATE OR REPLACE FUNCTION public.custom_access_token_hook(event jsonb)
RETURNS jsonb
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    uid uuid;
    original_claims jsonb;
    app_metadata jsonb;
    is_admin_user boolean;
BEGIN
    IF event->>'user_id' IS NULL OR event->'claims' IS NULL THEN
        RAISE LOG 'custom_access_token_hook: rejecting malformed event (user_id present=%, claims present=%)',
            (event ? 'user_id'), (event ? 'claims');
        RAISE EXCEPTION 'custom_access_token_hook: malformed event (user_id and claims are required)';
    END IF;

    uid := (event->>'user_id')::uuid;
    original_claims := event->'claims';
    app_metadata := COALESCE(original_claims->'app_metadata', '{}'::jsonb);

    SELECT EXISTS (
        SELECT 1
        FROM public.user_roles ur
        JOIN public.roles r ON r.id = ur.role_id
        WHERE ur.user_id = uid
          AND r.name = 'admin'
    ) INTO is_admin_user;

    IF is_admin_user THEN
        app_metadata := jsonb_set(app_metadata, '{role}', '"admin"'::jsonb, true);
    ELSE
        -- Clear any stale role claim so a revoked admin does not keep a stale
        -- token-side metadata entry on refresh.
        app_metadata := app_metadata - 'role';
    END IF;

    RETURN jsonb_build_object(
        'claims',
        jsonb_set(original_claims, '{app_metadata}', app_metadata, true)
    );
END;
$$;

-- ---------------------------------------------------------------------------
-- 4) Triggers (public.* first, then auth.users)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE ON public.users
    FOR EACH ROW
    EXECUTE FUNCTION public.set_users_updated_at();

CREATE OR REPLACE TRIGGER trg_cardgroups_set_updated_at
    BEFORE UPDATE ON public.cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cardgroups_updated_at();

CREATE OR REPLACE TRIGGER trg_cards_set_updated_at
    BEFORE UPDATE ON public.cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cards_updated_at();

CREATE OR REPLACE TRIGGER trg_user_card_fsrs_set_updated_at
    BEFORE UPDATE ON public.user_card_fsrs
    FOR EACH ROW
    EXECUTE FUNCTION public.set_user_card_fsrs_updated_at();

-- The lone trigger on auth.users: link new auth identities to public.users.
CREATE OR REPLACE TRIGGER trg_handle_new_user
    AFTER INSERT ON auth.users
    FOR EACH ROW
    EXECUTE FUNCTION public.handle_new_user();

-- ---------------------------------------------------------------------------
-- 5) Row Level Security
-- ---------------------------------------------------------------------------

ALTER TABLE public.users             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.roles             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cardgroups        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cards             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.swipe_records     ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.ping_records      ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_card_fsrs    ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_preferences  ENABLE ROW LEVEL SECURITY;

-- Policies are declared in the same order as the original migration history
-- so that any future audit grep finds them in the expected sequence.

DROP POLICY IF EXISTS users_select_own_or_admin ON public.users;
CREATE POLICY users_select_own_or_admin
    ON public.users
    FOR SELECT
    TO authenticated
    USING (id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS users_update_own_or_admin ON public.users;
CREATE POLICY users_update_own_or_admin
    ON public.users
    FOR UPDATE
    TO authenticated
    USING (id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS cardgroups_all_owner_or_admin ON public.cardgroups;
CREATE POLICY cardgroups_all_owner_or_admin
    ON public.cardgroups
    FOR ALL
    TO authenticated
    USING (owner_id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (owner_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS cards_all_owner_or_admin ON public.cards;
CREATE POLICY cards_all_owner_or_admin
    ON public.cards
    FOR ALL
    TO authenticated
    USING (
        cardgroup_id IN (
            SELECT id
            FROM public.cardgroups
            WHERE owner_id = auth.uid()
        )
        OR public.is_admin(auth.uid())
    )
    WITH CHECK (
        cardgroup_id IN (
            SELECT id
            FROM public.cardgroups
            WHERE owner_id = auth.uid()
        )
        OR public.is_admin(auth.uid())
    );

DROP POLICY IF EXISTS swipe_records_select_own_or_admin ON public.swipe_records;
CREATE POLICY swipe_records_select_own_or_admin
    ON public.swipe_records
    FOR SELECT
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS swipe_records_insert_own ON public.swipe_records;
CREATE POLICY swipe_records_insert_own
    ON public.swipe_records
    FOR INSERT
    TO authenticated
    WITH CHECK (user_id = auth.uid());

DROP POLICY IF EXISTS roles_select_public ON public.roles;
CREATE POLICY roles_select_public
    ON public.roles
    FOR SELECT
    TO PUBLIC
    USING (true);

DROP POLICY IF EXISTS roles_insert_admin ON public.roles;
CREATE POLICY roles_insert_admin
    ON public.roles
    FOR INSERT
    TO authenticated
    WITH CHECK (public.is_admin(auth.uid()));

DROP POLICY IF EXISTS roles_update_admin ON public.roles;
CREATE POLICY roles_update_admin
    ON public.roles
    FOR UPDATE
    TO authenticated
    USING (public.is_admin(auth.uid()))
    WITH CHECK (public.is_admin(auth.uid()));

DROP POLICY IF EXISTS roles_delete_admin ON public.roles;
CREATE POLICY roles_delete_admin
    ON public.roles
    FOR DELETE
    TO authenticated
    USING (public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_roles_select_self_or_admin ON public.user_roles;
CREATE POLICY user_roles_select_self_or_admin
    ON public.user_roles
    FOR SELECT
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_roles_insert_admin ON public.user_roles;
CREATE POLICY user_roles_insert_admin
    ON public.user_roles
    FOR INSERT
    TO authenticated
    WITH CHECK (public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_roles_update_admin ON public.user_roles;
CREATE POLICY user_roles_update_admin
    ON public.user_roles
    FOR UPDATE
    TO authenticated
    USING (public.is_admin(auth.uid()))
    WITH CHECK (public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_roles_delete_admin ON public.user_roles;
CREATE POLICY user_roles_delete_admin
    ON public.user_roles
    FOR DELETE
    TO authenticated
    USING (public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_card_fsrs_self ON public.user_card_fsrs;
CREATE POLICY user_card_fsrs_self
    ON public.user_card_fsrs
    FOR ALL
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (user_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_preferences_select_own_or_admin ON public.user_preferences;
CREATE POLICY user_preferences_select_own_or_admin
    ON public.user_preferences
    FOR SELECT
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_preferences_insert_own_or_admin ON public.user_preferences;
CREATE POLICY user_preferences_insert_own_or_admin
    ON public.user_preferences
    FOR INSERT
    TO authenticated
    WITH CHECK (user_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_preferences_update_own_or_admin ON public.user_preferences;
CREATE POLICY user_preferences_update_own_or_admin
    ON public.user_preferences
    FOR UPDATE
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (user_id = auth.uid() OR public.is_admin(auth.uid()));

DROP POLICY IF EXISTS user_preferences_delete_own_or_admin ON public.user_preferences;
CREATE POLICY user_preferences_delete_own_or_admin
    ON public.user_preferences
    FOR DELETE
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

-- ---------------------------------------------------------------------------
-- 6) Function privilege management
-- ---------------------------------------------------------------------------

-- is_admin: anonymous PostgREST callers must not probe role membership.
REVOKE ALL ON FUNCTION public.is_admin(uuid) FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT EXECUTE ON FUNCTION public.is_admin(uuid) TO authenticated;
    END IF;
END
$$;

-- custom_access_token_hook: only the Supabase auth role that mints tokens may
-- call this function. PostgREST callers (authenticated, anon) must never invoke
-- it. The test container is a plain PostgreSQL instance without these roles,
-- so the GRANT/REVOKE blocks are guarded with pg_roles probes to stay
-- idempotent across environments.
REVOKE ALL ON FUNCTION public.custom_access_token_hook(jsonb) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON FUNCTION public.custom_access_token_hook(jsonb) FROM authenticated;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON FUNCTION public.custom_access_token_hook(jsonb) FROM anon;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN
        GRANT EXECUTE ON FUNCTION public.custom_access_token_hook(jsonb) TO supabase_auth_admin;
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 7) Seed data
-- ---------------------------------------------------------------------------

INSERT INTO public.roles (name) VALUES ('admin')
    ON CONFLICT DO NOTHING;
INSERT INTO public.roles (name) VALUES ('general')
    ON CONFLICT DO NOTHING;

COMMIT;
