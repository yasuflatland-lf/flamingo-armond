-- Reverse 20260509000000_add_general_role.up.sql.
--
-- Removes only the seeded row, not the table. Any user_roles assignment
-- that referenced this role row is cascaded by the ON DELETE CASCADE on
-- the user_roles.role_id FK declared in 20260430080000_initial_schema.up.sql.

BEGIN;

DELETE FROM public.roles WHERE name = 'general';

COMMIT;
