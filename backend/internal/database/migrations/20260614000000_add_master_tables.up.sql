-- Master catalog persistence layer: master_cardgroups and master_cards.
--
-- These admin-curated tables back the public master catalog. Rows are authored
-- by admins and later copy-snapshotted into per-user public.cardgroups /
-- public.cards. They are independent of the per-user tables (no FK to users or
-- cardgroups) and are gated by admin-only RLS.
--
-- Order within this file mirrors the initial schema:
--   1. Trigger functions (plpgsql; late-bind). Created WITH a pinned, empty
--      search_path from the start (the function_search_path_mutable hardening
--      that 20260603090100 retrofitted onto the original set_*_updated_at
--      functions). now() resolves from pg_catalog, which is always on the path.
--   2. Tables in foreign-key order (master_cardgroups -> master_cards) with
--      per-table indexes interleaved.
--   3. Triggers (BEFORE UPDATE on each table).
--   4. RLS enable + admin-only policies. is_admin lives in the private schema
--      (moved there by 20260603090200), so policies reference private.is_admin.
--
-- These trigger functions assign only NEW.updated_at and reference no
-- schema-qualified objects, so they are SECURITY INVOKER (the default) and need
-- no GRANT/REVOKE management — they fire with the table owner's privileges and
-- are never reachable as PostgREST RPCs the way SECURITY DEFINER functions are.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) Trigger functions (plpgsql; late-bind). Pinned search_path from creation.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION public.set_master_cardgroups_updated_at()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = ''
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.set_master_cards_updated_at()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = ''
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- 2) Tables (FK order) + per-table indexes
-- ---------------------------------------------------------------------------

-- master_cardgroups: admin-curated catalog deck; referenced by master_cards.
CREATE TABLE IF NOT EXISTS public.master_cardgroups (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name               text        NOT NULL,
    description        text        NULL,
    language           text        NULL,
    level              text        NULL,
    category           text        NULL,
    cover_image_url    text        NULL,
    source             text        NULL,
    version            integer     NOT NULL DEFAULT 1,
    status             text        NOT NULL DEFAULT 'draft',
    is_default_starter boolean     NOT NULL DEFAULT false,
    sort_order         integer     NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT master_cardgroups_name_length   CHECK (char_length(trim(name)) BETWEEN 1 AND 100),
    CONSTRAINT master_cardgroups_status_values CHECK (status IN ('draft', 'published'))
);

CREATE INDEX IF NOT EXISTS idx_master_cardgroups_status_starter
    ON public.master_cardgroups (status, is_default_starter);
CREATE INDEX IF NOT EXISTS idx_master_cardgroups_sort
    ON public.master_cardgroups (sort_order, id);

-- master_cards: admin-curated cards owned 1:N by a master_cardgroup.
CREATE TABLE IF NOT EXISTS public.master_cards (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    master_cardgroup_id uuid        NOT NULL REFERENCES public.master_cardgroups(id) ON DELETE CASCADE,
    front               text        NOT NULL,
    back                text        NOT NULL,
    position            integer     NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT master_cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 500),
    CONSTRAINT master_cards_back_length  CHECK (char_length(trim(back))  BETWEEN 1 AND 500)
);

CREATE INDEX IF NOT EXISTS idx_master_cards_master_cardgroup_id
    ON public.master_cards (master_cardgroup_id);

-- Unique index supporting an ON CONFLICT (master_cardgroup_id, front) upsert.
CREATE UNIQUE INDEX IF NOT EXISTS uq_master_cards_cg_front
    ON public.master_cards (master_cardgroup_id, front);

-- ---------------------------------------------------------------------------
-- 3) Triggers (BEFORE UPDATE on each table)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE TRIGGER trg_master_cardgroups_set_updated_at
    BEFORE UPDATE ON public.master_cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_master_cardgroups_updated_at();

CREATE OR REPLACE TRIGGER trg_master_cards_set_updated_at
    BEFORE UPDATE ON public.master_cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_master_cards_updated_at();

-- ---------------------------------------------------------------------------
-- 4) Row Level Security: admin-only on both tables.
-- ---------------------------------------------------------------------------

ALTER TABLE public.master_cardgroups ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.master_cards      ENABLE ROW LEVEL SECURITY;

-- is_admin was moved out of public into the private schema by
-- 20260603090200_restrict_definer_function_exposure; reference it there.
DROP POLICY IF EXISTS master_cardgroups_admin_all ON public.master_cardgroups;
CREATE POLICY master_cardgroups_admin_all
    ON public.master_cardgroups
    FOR ALL
    TO authenticated
    USING (private.is_admin(auth.uid()))
    WITH CHECK (private.is_admin(auth.uid()));

DROP POLICY IF EXISTS master_cards_admin_all ON public.master_cards;
CREATE POLICY master_cards_admin_all
    ON public.master_cards
    FOR ALL
    TO authenticated
    USING (private.is_admin(auth.uid()))
    WITH CHECK (private.is_admin(auth.uid()));

COMMIT;
