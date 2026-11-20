-- Drop two single-column FK indexes that a unique composite already covers.
--
-- idx_cards_cardgroup_id (cardgroup_id) is the leading-column prefix of
-- uq_cards_cardgroup_front (cardgroup_id, front); idx_master_cards_master_cardgroup_id
-- (master_cardgroup_id) is the prefix of uq_master_cards_cg_front
-- (master_cardgroup_id, front). A btree serves an equality lookup on its leading
-- column, so FK checks, ON DELETE CASCADE and WHERE cardgroup_id = ? keep an index
-- path; the narrow indexes only add a write per insert/update on the two upsert
-- tables. Same reasoning as 20260703000000 for idx_swipe_records_user_id.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP INDEX IF EXISTS public.idx_cards_cardgroup_id;
DROP INDEX IF EXISTS public.idx_master_cards_master_cardgroup_id;

COMMIT;
