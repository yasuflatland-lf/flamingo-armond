ALTER TABLE public.master_cardgroups
    DROP CONSTRAINT IF EXISTS master_cardgroups_cover_image_url_length,
    DROP CONSTRAINT IF EXISTS master_cardgroups_source_length,
    DROP CONSTRAINT IF EXISTS master_cardgroups_category_length,
    DROP CONSTRAINT IF EXISTS master_cardgroups_level_length,
    DROP CONSTRAINT IF EXISTS master_cardgroups_language_length,
    DROP CONSTRAINT IF EXISTS master_cardgroups_description_length;
