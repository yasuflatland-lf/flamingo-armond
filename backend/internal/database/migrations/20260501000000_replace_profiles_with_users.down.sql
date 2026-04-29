DROP TRIGGER IF EXISTS trg_handle_new_user ON auth.users;
DROP FUNCTION IF EXISTS public.handle_new_user();
DROP TRIGGER IF EXISTS trg_users_set_updated_at ON public.users;
DROP FUNCTION IF EXISTS public.set_users_updated_at();

ALTER INDEX IF EXISTS idx_users_updated_at RENAME TO idx_profiles_updated_at;
ALTER TABLE IF EXISTS public.users RENAME TO profiles;
