DROP TRIGGER IF EXISTS trg_handle_new_user ON auth.users;
DROP FUNCTION IF EXISTS public.handle_new_user();
DROP TRIGGER IF EXISTS trg_profiles_set_updated_at ON public.profiles;
DROP FUNCTION IF EXISTS public.set_profiles_updated_at();
DROP TABLE IF EXISTS public.profiles;
