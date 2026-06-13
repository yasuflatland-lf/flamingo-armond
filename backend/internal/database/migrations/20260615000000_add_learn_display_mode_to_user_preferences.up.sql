-- 20260615000000_add_learn_display_mode_to_user_preferences.up.sql
--
-- Adds the learn_display_mode column to public.user_preferences.
-- The column controls whether cards are shown in flip-to-reveal (default)
-- or always-visible mode during a learn session.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.user_preferences
  ADD COLUMN learn_display_mode text NOT NULL DEFAULT 'flip_to_reveal'
    CHECK (learn_display_mode IN ('flip_to_reveal', 'always_visible'));

COMMIT;
