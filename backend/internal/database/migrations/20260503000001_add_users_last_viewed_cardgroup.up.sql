-- Add public.users.last_viewed_cardgroup_id, a self-referential nullable FK to
-- public.cardgroups. Used to land returning users directly on the learn screen
-- for the cardgroup they most recently viewed.
--
-- ON DELETE SET NULL: when the referenced cardgroup is deleted, the column is
-- nulled out rather than cascading the delete back to the user row. The
-- companion index keeps that cascade from full-scanning users on every
-- cardgroup delete.
--
-- The existing users_update_own_or_admin RLS policy already covers this column
-- (id = auth.uid() OR is_admin()), so no new policy is required.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN last_viewed_cardgroup_id uuid NULL
        REFERENCES public.cardgroups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_users_last_viewed_cardgroup_id
    ON public.users (last_viewed_cardgroup_id);

COMMIT;
