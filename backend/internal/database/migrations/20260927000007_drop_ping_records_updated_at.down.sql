BEGIN;

ALTER TABLE public.ping_records
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

COMMIT;
