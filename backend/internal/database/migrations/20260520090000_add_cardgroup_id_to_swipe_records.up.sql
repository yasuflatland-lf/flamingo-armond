-- Denormalize the cardgroup at swipe time so swipe_records can be queried
-- without joining the cards aggregate. Pre-launch DB reset guarantees
-- public.swipe_records is empty when this NOT NULL column is added.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.swipe_records
    ADD COLUMN cardgroup_id uuid NOT NULL;

CREATE INDEX IF NOT EXISTS idx_swipe_records_user_cardgroup
    ON public.swipe_records (user_id, cardgroup_id, reviewed_at DESC);

COMMIT;
