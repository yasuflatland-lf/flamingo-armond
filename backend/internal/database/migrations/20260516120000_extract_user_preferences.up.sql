-- Extract user preference state (last_viewed_cardgroup_id) from public.users
-- into a dedicated public.user_preferences table.
--
-- Rationale: the field describes UI continuity (behavioural state), not user
-- identity. Co-locating it on users forces the User repository to reference the
-- cardgroups table, violating the "repository per aggregate" principle and
-- importing pgconn solely for FK classification.
--
-- Migration strategy: single-deploy cutover.
--   1. Create user_preferences with the same FK and ON DELETE SET NULL semantics.
--   2. Backfill rows from users where last_viewed_cardgroup_id IS NOT NULL.
--   3. Drop the index and column from users.
--   4. Enable RLS on user_preferences with the same "self-or-admin" policies
--      used on adjacent per-user tables (user_card_fsrs, users).
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE TABLE public.user_preferences (
    user_id                  uuid PRIMARY KEY
        REFERENCES public.users(id) ON DELETE CASCADE,
    last_viewed_cardgroup_id uuid NULL
        REFERENCES public.cardgroups(id) ON DELETE SET NULL,
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_preferences_last_viewed_cardgroup_id
    ON public.user_preferences (last_viewed_cardgroup_id);

-- Backfill: carry over any existing preference values before dropping the column.
INSERT INTO public.user_preferences (user_id, last_viewed_cardgroup_id, updated_at)
SELECT id, last_viewed_cardgroup_id, COALESCE(updated_at, now())
FROM   public.users
WHERE  last_viewed_cardgroup_id IS NOT NULL;

-- Remove the column (and its index) from users now that the data is migrated.
DROP INDEX IF EXISTS public.idx_users_last_viewed_cardgroup_id;
ALTER TABLE public.users DROP COLUMN IF EXISTS last_viewed_cardgroup_id;

-- RLS: same "self-or-admin" shape as users and user_card_fsrs.
ALTER TABLE public.user_preferences ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_preferences_select_own_or_admin
    ON public.user_preferences
    FOR SELECT
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY user_preferences_insert_own_or_admin
    ON public.user_preferences
    FOR INSERT
    TO authenticated
    WITH CHECK (user_id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY user_preferences_update_own_or_admin
    ON public.user_preferences
    FOR UPDATE
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (user_id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY user_preferences_delete_own_or_admin
    ON public.user_preferences
    FOR DELETE
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

COMMIT;
