CREATE TABLE IF NOT EXISTS public.swipe_records (
    id             uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid             NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    card_id        uuid             NOT NULL REFERENCES public.cards(id) ON DELETE CASCADE,
    rating         integer          NOT NULL CHECK (rating BETWEEN 1 AND 4),
    reviewed_at    timestamptz      NOT NULL DEFAULT now(),

    due            timestamptz      NOT NULL,
    stability      double precision NOT NULL,
    difficulty     double precision NOT NULL,
    elapsed_days   integer          NOT NULL,
    scheduled_days integer          NOT NULL,
    reps           integer          NOT NULL,
    lapses         integer          NOT NULL,
    state          integer          NOT NULL,
    last_review    timestamptz      NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_swipe_records_user_id       ON public.swipe_records (user_id);
CREATE INDEX IF NOT EXISTS idx_swipe_records_card_id       ON public.swipe_records (card_id);
CREATE INDEX IF NOT EXISTS idx_swipe_records_user_reviewed ON public.swipe_records (user_id, reviewed_at DESC);
