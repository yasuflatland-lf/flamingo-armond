-- Initial public schema for flamingo-armond.
--
-- Creates every public table the Go backend touches, in dependency order:
-- users → roles → user_roles → cardgroups → cards → swipe_records.
-- public.users is 1:1 with auth.users, populated by the handle_new_user
-- trigger that fires on auth.users insert. After all tables are in place,
-- Row Level Security is enabled with zero policies on the public schema so
-- PostgREST callers (anon, authenticated) hit PostgreSQL's default-deny;
-- the backend connects as the table-owner role and bypasses RLS unless
-- FORCE ROW LEVEL SECURITY is set, so application queries and migrations
-- are unaffected. See docs/backend.md "Authorization at the usecase layer".

-- ---------------------------------------------------------------------------
-- users (1:1 with auth.users)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.users (
    id            uuid PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    display_name  text,
    bio           text,
    avatar_url    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_updated_at ON public.users (updated_at);

CREATE OR REPLACE FUNCTION public.set_users_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE ON public.users
    FOR EACH ROW
    EXECUTE FUNCTION public.set_users_updated_at();

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

DROP TRIGGER IF EXISTS trg_handle_new_user ON auth.users;
CREATE TRIGGER trg_handle_new_user
    AFTER INSERT ON auth.users
    FOR EACH ROW
    EXECUTE FUNCTION public.handle_new_user();

-- ---------------------------------------------------------------------------
-- roles + user_roles (M:N)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.roles (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE
);

INSERT INTO public.roles (name) VALUES ('admin')
    ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS public.user_roles (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES public.roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON public.user_roles (role_id);

-- ---------------------------------------------------------------------------
-- cardgroups (1:N owned by users)
-- ---------------------------------------------------------------------------

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

CREATE OR REPLACE FUNCTION public.set_cardgroups_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_cardgroups_set_updated_at
    BEFORE UPDATE ON public.cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cardgroups_updated_at();

-- ---------------------------------------------------------------------------
-- cards (FSRSState persisted as a flat column block)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.cards (
    id              uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    cardgroup_id    uuid             NOT NULL REFERENCES public.cardgroups(id) ON DELETE CASCADE,
    front           text             NOT NULL,
    back            text             NOT NULL,
    due             timestamptz      NOT NULL,
    stability       double precision NOT NULL,
    difficulty      double precision NOT NULL,
    elapsed_days    integer          NOT NULL,
    scheduled_days  integer          NOT NULL,
    reps            integer          NOT NULL,
    lapses          integer          NOT NULL,
    state           integer          NOT NULL,
    last_review     timestamptz      NOT NULL,
    created_at      timestamptz      NOT NULL DEFAULT now(),
    updated_at      timestamptz      NOT NULL DEFAULT now(),
    CONSTRAINT cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 500),
    CONSTRAINT cards_back_length  CHECK (char_length(trim(back))  BETWEEN 1 AND 500)
);

CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_id  ON public.cards (cardgroup_id);
CREATE INDEX IF NOT EXISTS idx_cards_due           ON public.cards (due);
CREATE INDEX IF NOT EXISTS idx_cards_state         ON public.cards (state);
CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_due ON public.cards (cardgroup_id, due);

CREATE OR REPLACE FUNCTION public.set_cards_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_cards_set_updated_at
    BEFORE UPDATE ON public.cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cards_updated_at();

-- ---------------------------------------------------------------------------
-- swipe_records (immutable review log; no updated_at)
-- ---------------------------------------------------------------------------

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

-- ---------------------------------------------------------------------------
-- Row Level Security: enable with zero policies on every public table.
-- PostgREST callers default-deny; the table-owner role bypasses RLS so the
-- Go backend is unaffected. schema_migrations is created and owned by
-- golang-migrate via the same connection, so enabling RLS here does not
-- interfere with the migration runner's bookkeeping writes.
-- ---------------------------------------------------------------------------

ALTER TABLE public.users             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.roles             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cardgroups        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cards             ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.swipe_records     ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.schema_migrations ENABLE ROW LEVEL SECURITY;
