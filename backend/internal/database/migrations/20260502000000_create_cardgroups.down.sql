-- Drop cardgroups table and its updated_at trigger / function.

DROP TRIGGER  IF EXISTS trg_cardgroups_set_updated_at ON public.cardgroups;
DROP FUNCTION IF EXISTS public.set_cardgroups_updated_at();
DROP TABLE    IF EXISTS public.cardgroups;
