-- Restore the previous update-only behavior for updated_at triggers.

CREATE OR REPLACE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE ON public.users
    FOR EACH ROW
    EXECUTE FUNCTION public.set_users_updated_at();

CREATE OR REPLACE TRIGGER trg_cardgroups_set_updated_at
    BEFORE UPDATE ON public.cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cardgroups_updated_at();

CREATE OR REPLACE TRIGGER trg_cards_set_updated_at
    BEFORE UPDATE ON public.cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_cards_updated_at();

CREATE OR REPLACE TRIGGER trg_user_card_fsrs_set_updated_at
    BEFORE UPDATE ON public.user_card_fsrs
    FOR EACH ROW
    EXECUTE FUNCTION public.set_user_card_fsrs_updated_at();

CREATE OR REPLACE TRIGGER trg_master_cardgroups_set_updated_at
    BEFORE UPDATE ON public.master_cardgroups
    FOR EACH ROW
    EXECUTE FUNCTION public.set_master_cardgroups_updated_at();

CREATE OR REPLACE TRIGGER trg_master_cards_set_updated_at
    BEFORE UPDATE ON public.master_cards
    FOR EACH ROW
    EXECUTE FUNCTION public.set_master_cards_updated_at();
