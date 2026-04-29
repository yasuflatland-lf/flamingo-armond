-- Drop cards table and its updated_at trigger / function.

DROP TRIGGER  IF EXISTS trg_cards_set_updated_at ON public.cards;
DROP FUNCTION IF EXISTS public.set_cards_updated_at();
DROP TABLE    IF EXISTS public.cards;
