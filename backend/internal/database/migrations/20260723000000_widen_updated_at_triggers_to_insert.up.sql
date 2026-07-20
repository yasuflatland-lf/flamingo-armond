-- Keep updated_at on one clock by assigning it in the database for both
-- inserts and updates. The existing trigger functions remain unchanged.

CREATE OR REPLACE TRIGGER trg_users_set_updated_at
    BEFORE INSERT OR UPDATE ON public.users
    FOR EACH ROW
    EXECUTE FUNCTION public.set_users_updated_at();

CREATE OR REPLACE TRIGGER trg_cardgroups_set_updated_at
    BEFORE INSERT OR UPDATE ON public.cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cardgroups_updated_at();

CREATE OR REPLACE TRIGGER trg_cards_set_updated_at
    BEFORE INSERT OR UPDATE ON public.cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cards_updated_at();

CREATE OR REPLACE TRIGGER trg_user_card_fsrs_set_updated_at
    BEFORE INSERT OR UPDATE ON public.user_card_fsrs
    FOR EACH ROW
    EXECUTE FUNCTION public.set_user_card_fsrs_updated_at();

CREATE OR REPLACE TRIGGER trg_master_cardgroups_set_updated_at
    BEFORE INSERT OR UPDATE ON public.master_cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_master_cardgroups_updated_at();

CREATE OR REPLACE TRIGGER trg_master_cards_set_updated_at
    BEFORE INSERT OR UPDATE ON public.master_cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_master_cards_updated_at();
