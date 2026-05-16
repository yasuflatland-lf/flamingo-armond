-- Reverse of 20260516120000_extract_user_preferences.up.sql.
--
-- Re-adds last_viewed_cardgroup_id to public.users, backfills values from
-- public.user_preferences, then drops the user_preferences table (which also
-- implicitly removes all associated RLS policies and indexes).
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN last_viewed_cardgroup_id uuid NULL
        REFERENCES public.cardgroups(id) ON DELETE SET NULL;

CREATE INDEX idx_users_last_viewed_cardgroup_id
    ON public.users (last_viewed_cardgroup_id);

-- Restore preference values to the users column before dropping the source table.
UPDATE public.users u
SET    last_viewed_cardgroup_id = p.last_viewed_cardgroup_id
FROM   public.user_preferences p
WHERE  u.id = p.user_id
  AND  p.last_viewed_cardgroup_id IS NOT NULL;

-- DROP TABLE removes all indexes, triggers, and RLS policies on the table.
DROP TABLE public.user_preferences;

COMMIT;
