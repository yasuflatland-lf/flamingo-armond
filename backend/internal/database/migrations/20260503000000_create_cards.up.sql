-- Introduce cards and persist FSRSState as a flat column block.

CREATE TABLE IF NOT EXISTS public.cards (
    id              uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    cardgroup_id    uuid             NOT NULL REFERENCES public.cardgroups(id) ON DELETE CASCADE,
    front           text             NOT NULL,
    back            text             NOT NULL,
    due             timestamptz      NOT NULL,
    stability       double precision NOT NULL,
    difficulty      double precision NOT NULL,
    elapsed_days    integer          NOT NULL,
    scheduled_days  integer          NOT NULL,
    reps            integer          NOT NULL,
    lapses          integer          NOT NULL,
    state           integer          NOT NULL,
    last_review     timestamptz      NOT NULL,
    created_at      timestamptz      NOT NULL DEFAULT now(),
    updated_at      timestamptz      NOT NULL DEFAULT now(),
    CONSTRAINT cards_front_length CHECK (char_length(trim(front)) BETWEEN 1 AND 500),
    CONSTRAINT cards_back_length  CHECK (char_length(trim(back))  BETWEEN 1 AND 500)
);

CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_id  ON public.cards (cardgroup_id);
CREATE INDEX IF NOT EXISTS idx_cards_due           ON public.cards (due);
CREATE INDEX IF NOT EXISTS idx_cards_state         ON public.cards (state);
CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_due ON public.cards (cardgroup_id, due);

CREATE OR REPLACE FUNCTION public.set_cards_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_cards_set_updated_at
    BEFORE UPDATE ON public.cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cards_updated_at();
