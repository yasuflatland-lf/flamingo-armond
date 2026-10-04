BEGIN;

CREATE INDEX IF NOT EXISTS idx_cardgroups_owner_id   ON public.cardgroups (owner_id);
CREATE INDEX IF NOT EXISTS idx_cardgroups_updated_at ON public.cardgroups (updated_at);

DROP INDEX IF EXISTS public.idx_cardgroups_owner_updated_at_id;

COMMIT;
