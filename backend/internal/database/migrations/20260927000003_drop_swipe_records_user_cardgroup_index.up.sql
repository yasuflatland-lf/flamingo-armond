-- Drop idx_swipe_records_user_cardgroup (user_id, cardgroup_id, reviewed_at DESC).
-- It served only SwipeRecordRepository.FindByUserAndCardgroup, which has been
-- removed. The comments in 20260703000000_index_hygiene_users_swipe_records
-- and 20260721000000_add_cardgroup_fk_to_swipe_records that list this index as
-- a surviving composite predate its removal. user_id-keyed access paths (RLS,
-- ON DELETE CASCADE, ListByUserSince) are served by
-- idx_swipe_records_user_reviewed; cardgroup_id-keyed paths by
-- idx_swipe_records_cardgroup_id.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP INDEX IF EXISTS public.idx_swipe_records_user_cardgroup;

COMMIT;
