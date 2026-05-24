-- Add optimistic concurrency versioning to public.users.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN version bigint NOT NULL DEFAULT 0;

COMMIT;
