-- Extract per-user FSRS scheduling state out of cards.
--
-- Destructive by design for this early-stage app: existing per-card progress
-- is discarded. Do not apply automatically from implementation scripts.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

DROP INDEX IF EXISTS public.idx_cards_cardgroup_due;
DROP INDEX IF EXISTS public.idx_cards_state;
DROP INDEX IF EXISTS public.idx_cards_due;

ALTER TABLE public.cards
    DROP COLUMN due,
    DROP COLUMN stability,
    DROP COLUMN difficulty,
    DROP COLUMN elapsed_days,
    DROP COLUMN scheduled_days,
    DROP COLUMN reps,
    DROP COLUMN lapses,
    DROP COLUMN state,
    DROP COLUMN last_review;

CREATE TABLE public.user_card_fsrs (
    user_id        uuid             NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    card_id        uuid             NOT NULL REFERENCES public.cards(id) ON DELETE CASCADE,
    state          integer          NOT NULL,
    due            timestamptz      NOT NULL,
    stability      double precision NOT NULL,
    difficulty     double precision NOT NULL,
    reps           integer          NOT NULL,
    lapses         integer          NOT NULL,
    last_review    timestamptz      NOT NULL,
    elapsed_days   integer          NOT NULL,
    scheduled_days integer          NOT NULL,
    created_at     timestamptz      NOT NULL DEFAULT now(),
    updated_at     timestamptz      NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, card_id)
);

CREATE INDEX idx_user_card_fsrs_user_due
    ON public.user_card_fsrs (user_id, due);

CREATE OR REPLACE FUNCTION public.set_user_card_fsrs_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_user_card_fsrs_set_updated_at
    BEFORE UPDATE ON public.user_card_fsrs
    FOR EACH ROW
    EXECUTE FUNCTION public.set_user_card_fsrs_updated_at();

ALTER TABLE public.user_card_fsrs ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_card_fsrs_self
    ON public.user_card_fsrs
    FOR ALL
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (user_id = auth.uid() OR public.is_admin(auth.uid()));

COMMIT;
