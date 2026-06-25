BEGIN;

-- Drop the unused master_cardgroups metadata columns. The admin catalog no
-- longer exposes language / level / category / cover_image_url / source; the
-- aggregate, repository, and GraphQL schema have removed these fields, so the
-- columns are now dead. DROP COLUMN IF EXISTS keeps the migration idempotent.
ALTER TABLE public.master_cardgroups DROP COLUMN IF EXISTS language;
ALTER TABLE public.master_cardgroups DROP COLUMN IF EXISTS level;
ALTER TABLE public.master_cardgroups DROP COLUMN IF EXISTS category;
ALTER TABLE public.master_cardgroups DROP COLUMN IF EXISTS cover_image_url;
ALTER TABLE public.master_cardgroups DROP COLUMN IF EXISTS source;

COMMIT;
