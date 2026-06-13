-- Reverse of 20260614000000_add_master_tables.up.sql.
--
-- Dependency-reverse order: drop policies, disable RLS, drop triggers, the
-- unique index (IF EXISTS — idempotent), then tables (FK-referenced ones
-- last: master_cards before master_cardgroups), and finally the trigger
-- functions the triggers referenced. The set_master_*_updated_at functions are
-- owned exclusively by these two tables — no other table reuses them — so
-- dropping them here removes no shared object. Every DROP is IF EXISTS; no
-- CASCADE — explicit ordering is safer than letting CASCADE silently pull in
-- objects added by future code. private.is_admin is NOT touched: it is a shared
-- helper still used by the rest of the schema's RLS policies.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1) Policies (reverse of Up).
-- ---------------------------------------------------------------------------

DROP POLICY IF EXISTS master_cards_admin_all      ON public.master_cards;
DROP POLICY IF EXISTS master_cardgroups_admin_all ON public.master_cardgroups;

-- ---------------------------------------------------------------------------
-- 2) Disable RLS (formal symmetry with Up; DROP TABLE below would also clear
--    the flag, but the explicit DISABLE keeps the rollback observable on a
--    partially-applied database).
-- ---------------------------------------------------------------------------

ALTER TABLE IF EXISTS public.master_cards      DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public.master_cardgroups DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 3) Triggers.
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_master_cards_set_updated_at      ON public.master_cards;
DROP TRIGGER IF EXISTS trg_master_cardgroups_set_updated_at ON public.master_cardgroups;

-- ---------------------------------------------------------------------------
-- 4) Explicit index drop for the unique index backing the ON CONFLICT upsert
--    contract. Plain idx_* indexes are implied by DROP TABLE, but naming this
--    one explicitly makes the rollback observable and intentional.
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS public.uq_master_cards_cg_front;

-- ---------------------------------------------------------------------------
-- 5) Tables (FK-referenced ones last).
-- ---------------------------------------------------------------------------

DROP TABLE IF EXISTS public.master_cards;
DROP TABLE IF EXISTS public.master_cardgroups;

-- ---------------------------------------------------------------------------
-- 6) Trigger functions (after every dependent object is gone). These are owned
--    solely by the master tables; private.is_admin is a shared helper and is
--    deliberately left in place.
-- ---------------------------------------------------------------------------

DROP FUNCTION IF EXISTS public.set_master_cards_updated_at();
DROP FUNCTION IF EXISTS public.set_master_cardgroups_updated_at();

COMMIT;
