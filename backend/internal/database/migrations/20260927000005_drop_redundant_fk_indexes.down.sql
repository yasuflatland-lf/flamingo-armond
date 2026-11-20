BEGIN;

CREATE INDEX IF NOT EXISTS idx_cards_cardgroup_id
    ON public.cards (cardgroup_id);
CREATE INDEX IF NOT EXISTS idx_master_cards_master_cardgroup_id
    ON public.master_cards (master_cardgroup_id);

COMMIT;
