-- 20260721000000_add_cardgroup_fk_to_swipe_records.up.sql
--
-- Teach the database that public.swipe_records.cardgroup_id references a deck.
-- The column was added as a bare uuid, so the sibling columns on the same table
-- (user_id, card_id) carry ON DELETE CASCADE foreign keys while the deck
-- reference carries none. Today the data stays consistent only by application
-- convention -- HandleSwipe copies the value from the locked card row and a card
-- cannot move between decks -- but the RLS INSERT policy
-- (swipe_records_insert_own) checks user_id alone, so an arbitrary deck id can be
-- planted through the data API, and deleting a deck leaves its review history
-- behind with a dangling reference.
--
-- ON DELETE CASCADE matches the sibling user_id / card_id foreign keys: a review
-- record is meaningless once the deck it was recorded against is gone, and
-- deleting a deck already cascades to its cards and, through them, to the same
-- swipe rows.
--
-- Pre-flight orphan cleanup. ALTER TABLE ... ADD CONSTRAINT validates every
-- existing row, so an environment that somehow accumulated an orphan would fail
-- the migration outright. The DELETE below removes exactly those rows; they are
-- unreachable by every read path (each swipe query is scoped by a deck that
-- exists) and are therefore garbage, not history:
--
--   DELETE FROM public.swipe_records
--   WHERE cardgroup_id NOT IN (SELECT id FROM public.cardgroups);
--
-- Index rationale. The index-hygiene migration
-- (20260703000000_index_hygiene_users_swipe_records) dropped
-- idx_swipe_records_user_id because both surviving composites lead with user_id
-- and therefore already serve every user_id-keyed access path. That argument does
-- not extend to cardgroup_id: the only index touching the column is
-- idx_swipe_records_user_cardgroup (user_id, cardgroup_id, reviewed_at DESC),
-- whose leading column is user_id, so a lookup or cascade keyed on cardgroup_id
-- alone has no usable left prefix. The new single-column index is the backing
-- index the foreign key needs, not a redundant duplicate of a composite.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DELETE FROM public.swipe_records
WHERE cardgroup_id NOT IN (SELECT id FROM public.cardgroups);

CREATE INDEX IF NOT EXISTS idx_swipe_records_cardgroup_id
    ON public.swipe_records (cardgroup_id);

ALTER TABLE public.swipe_records
    ADD CONSTRAINT swipe_records_cardgroup_id_fkey
    FOREIGN KEY (cardgroup_id) REFERENCES public.cardgroups(id) ON DELETE CASCADE;

COMMIT;
