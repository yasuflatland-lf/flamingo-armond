-- Widen the code-point upper bound of the text-length CHECK constraints so the
-- database backstop stops rejecting realistic emoji-rich values the domain layer
-- accepts. A grapheme cluster admits an unbounded combining-mark sequence, so no
-- finite code-point bound closes the gap entirely; the SQLSTATE 23514 arm in
-- internal/repository/pgerr.go classifies the residue as BAD_USER_INPUT instead
-- of an unexplained internal error.
--
-- The domain layer (internal/domain/card_text.go, internal/domain/cardgroup_name.go)
-- and the frontend (frontend/src/schemas/grapheme.ts) both count *grapheme
-- clusters*, while Postgres char_length() counts *code points*. A single ZWJ
-- family emoji is one grapheme but seven code points, so a value at exactly the
-- 500-grapheme card cap could carry 3500 code points and trip the old
-- BETWEEN 1 AND 500 bound. That rejection surfaced as an unexplained internal
-- error because SQLSTATE 23514 was classified nowhere in the repository layer.
--
-- The new upper bound is 20x the grapheme cap. The multiplier is sized from the
-- worst realistic expansion rather than a round number: a four-person ZWJ family
-- is 7 code points per grapheme, and 11 once every member carries a skin-tone
-- modifier, so a 500-grapheme card back can legitimately reach 5500 code points.
-- A 4x bound would still reject it. 20x leaves headroom for further combining
-- marks and variation selectors while keeping the constraint a meaningful
-- storage sanity check.
--
-- Widening is safe in the only direction that matters: every value the domain
-- accepts is at most `cap` graphemes, and no value the widened CHECK admits is
-- one the domain would have rejected -- the domain still runs first and remains
-- the single user-visible rule.
--
-- The lower bound stays at 1. "Non-empty after trim" is identical under both
-- counting rules, so there is nothing to widen there.

ALTER TABLE public.cardgroups
    DROP CONSTRAINT IF EXISTS cardgroups_name_length,
    ADD CONSTRAINT cardgroups_name_length CHECK (char_length(trim(name)) BETWEEN 1 AND 2000);

ALTER TABLE public.cards
    DROP CONSTRAINT IF EXISTS cards_front_length,
    ADD CONSTRAINT cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 10000),
    DROP CONSTRAINT IF EXISTS cards_back_length,
    ADD CONSTRAINT cards_back_length CHECK (char_length(trim(back)) BETWEEN 1 AND 10000);

ALTER TABLE public.master_cardgroups
    DROP CONSTRAINT IF EXISTS master_cardgroups_name_length,
    ADD CONSTRAINT master_cardgroups_name_length CHECK (char_length(trim(name)) BETWEEN 1 AND 2000);

ALTER TABLE public.master_cards
    DROP CONSTRAINT IF EXISTS master_cards_front_length,
    ADD CONSTRAINT master_cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 10000),
    DROP CONSTRAINT IF EXISTS master_cards_back_length,
    ADD CONSTRAINT master_cards_back_length CHECK (char_length(trim(back)) BETWEEN 1 AND 10000);
