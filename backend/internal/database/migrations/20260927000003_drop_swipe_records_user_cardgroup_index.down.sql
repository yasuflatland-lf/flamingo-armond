-- Reverse of 20260927000003_drop_swipe_records_user_cardgroup_index.up.sql.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_swipe_records_user_cardgroup
    ON public.swipe_records (user_id, cardgroup_id, reviewed_at DESC);

COMMIT;
