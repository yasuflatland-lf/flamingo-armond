-- Re-add legacy per-card FSRS columns and drop per-user FSRS state.
-- Data in user_card_fsrs is not restored to cards.

BEGIN;

DROP TABLE IF EXISTS public.user_card_fsrs;
DROP FUNCTION IF EXISTS public.set_user_card_fsrs_updated_at();

ALTER TABLE public.cards
    ADD COLUMN due             timestamptz      NOT NULL DEFAULT now(),
    ADD COLUMN stability       double precision NOT NULL DEFAULT 2.5,
    ADD COLUMN difficulty      double precision NOT NULL DEFAULT 5.0,
    ADD COLUMN elapsed_days    integer          NOT NULL DEFAULT 0,
    ADD COLUMN scheduled_days  integer          NOT NULL DEFAULT 0,
    ADD COLUMN reps            integer          NOT NULL DEFAULT 0,
    ADD COLUMN lapses          integer          NOT NULL DEFAULT 0,
    ADD COLUMN state           integer          NOT NULL DEFAULT 0,
    ADD COLUMN last_review     timestamptz      NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_cards_due
    ON public.cards (due);
CREATE INDEX IF NOT EXISTS idx_cards_state
    ON public.cards (state);
CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_due
    ON public.cards (cardgroup_id, due);

COMMIT;
