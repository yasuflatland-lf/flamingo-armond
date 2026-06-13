-- Add the missing foreign-key-backing index on public.user_card_fsrs.card_id.
--
-- user_card_fsrs has a composite primary key (user_id, card_id). A btree on that
-- PK serves lookups by user_id (left prefix) or (user_id, card_id), but NOT by
-- card_id alone. The foreign key card_id -> public.cards(id) is ON DELETE CASCADE,
-- so deleting a card (directly, via cardgroup cascade, or via the
-- auth.users -> public.users -> cardgroups -> cards delete-user cascade) must find
-- every user_card_fsrs row with that card_id. Without this index that lookup is a
-- sequential scan of the whole table (all users' rows), executed inside the
-- deleting transaction while it holds locks. Every other foreign key in the schema
-- already has a backing index; this one was missed because the composite PK
-- appears to cover card_id but does not.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_user_card_fsrs_card_id
    ON public.user_card_fsrs (card_id);

COMMIT;
