-- Introduce cardgroups: 1:N owned by users, with name length CHECK and updated_at trigger.

CREATE TABLE IF NOT EXISTS public.cardgroups (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid        NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name        text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cardgroups_name_length CHECK (char_length(trim(name)) BETWEEN 1 AND 100)
);

CREATE INDEX IF NOT EXISTS idx_cardgroups_owner_id   ON public.cardgroups (owner_id);
CREATE INDEX IF NOT EXISTS idx_cardgroups_updated_at ON public.cardgroups (updated_at);

CREATE OR REPLACE FUNCTION public.set_cardgroups_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_cardgroups_set_updated_at
    BEFORE UPDATE ON public.cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cardgroups_updated_at();
