-- Backstop length bounds for the master_cardgroups free-form text fields,
-- mirroring the existing master_cardgroups_name_length CHECK. Domain VOs
-- (ParseBoundedText / ParseCoverImageURL) enforce the same caps with
-- grapheme-cluster accuracy; these DB CHECKs protect API-direct callers.
-- cover_image_url is length-only here; URL format + scheme is the domain/Zod's
-- responsibility (a URL-format CHECK regex is brittle).
ALTER TABLE public.master_cardgroups
    ADD CONSTRAINT master_cardgroups_description_length
        CHECK (description IS NULL OR char_length(trim(description)) <= 1000),
    ADD CONSTRAINT master_cardgroups_language_length
        CHECK (language IS NULL OR char_length(trim(language)) <= 50),
    ADD CONSTRAINT master_cardgroups_level_length
        CHECK (level IS NULL OR char_length(trim(level)) <= 50),
    ADD CONSTRAINT master_cardgroups_category_length
        CHECK (category IS NULL OR char_length(trim(category)) <= 100),
    ADD CONSTRAINT master_cardgroups_source_length
        CHECK (source IS NULL OR char_length(trim(source)) <= 500),
    ADD CONSTRAINT master_cardgroups_cover_image_url_length
        CHECK (cover_image_url IS NULL OR char_length(trim(cover_image_url)) <= 2048);
