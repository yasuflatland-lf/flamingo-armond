-- Restore the original code-point upper bounds (100 for deck names, 500 for card
-- text). This narrows the constraint, so it fails if any row was stored with a
-- code-point length between the old and the new bound -- which is exactly the
-- emoji-rich content the up migration exists to admit. Truncate or repair those
-- rows before rolling back.

ALTER TABLE public.cardgroups
    DROP CONSTRAINT IF EXISTS cardgroups_name_length,
    ADD CONSTRAINT cardgroups_name_length CHECK (char_length(trim(name)) BETWEEN 1 AND 100);

ALTER TABLE public.cards
    DROP CONSTRAINT IF EXISTS cards_front_length,
    ADD CONSTRAINT cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 500),
    DROP CONSTRAINT IF EXISTS cards_back_length,
    ADD CONSTRAINT cards_back_length CHECK (char_length(trim(back)) BETWEEN 1 AND 500);

ALTER TABLE public.master_cardgroups
    DROP CONSTRAINT IF EXISTS master_cardgroups_name_length,
    ADD CONSTRAINT master_cardgroups_name_length CHECK (char_length(trim(name)) BETWEEN 1 AND 100);

ALTER TABLE public.master_cards
    DROP CONSTRAINT IF EXISTS master_cards_front_length,
    ADD CONSTRAINT master_cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 500),
    DROP CONSTRAINT IF EXISTS master_cards_back_length,
    ADD CONSTRAINT master_cards_back_length CHECK (char_length(trim(back)) BETWEEN 1 AND 500);
