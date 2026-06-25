BEGIN;

-- Structure-only reverse of the metadata-column drop. The columns are
-- re-created as nullable text exactly as add_master_tables originally declared
-- them; the dropped row data is NOT restored. ADD COLUMN IF NOT EXISTS keeps the
-- migration idempotent.
ALTER TABLE public.master_cardgroups ADD COLUMN IF NOT EXISTS language        text NULL;
ALTER TABLE public.master_cardgroups ADD COLUMN IF NOT EXISTS level           text NULL;
ALTER TABLE public.master_cardgroups ADD COLUMN IF NOT EXISTS category        text NULL;
ALTER TABLE public.master_cardgroups ADD COLUMN IF NOT EXISTS cover_image_url text NULL;
ALTER TABLE public.master_cardgroups ADD COLUMN IF NOT EXISTS source          text NULL;

COMMIT;
