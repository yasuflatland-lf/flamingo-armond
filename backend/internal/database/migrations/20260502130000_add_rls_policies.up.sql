-- Add owner-scoped Row Level Security policies for direct authenticated access.
--
-- The Go backend connects as the table owner and continues to bypass RLS. These
-- policies define the baseline for Supabase PostgREST / Edge callers that use
-- the authenticated role.
--
-- golang-migrate pgx/v5 does NOT auto-wrap migrations in a transaction; the
-- explicit BEGIN/COMMIT below ensures all-or-nothing execution.

BEGIN;

CREATE POLICY users_select_own_or_admin
    ON public.users
    FOR SELECT
    TO authenticated
    USING (id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY users_update_own_or_admin
    ON public.users
    FOR UPDATE
    TO authenticated
    USING (id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY cardgroups_all_owner_or_admin
    ON public.cardgroups
    FOR ALL
    TO authenticated
    USING (owner_id = auth.uid() OR public.is_admin(auth.uid()))
    WITH CHECK (owner_id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY cards_all_owner_or_admin
    ON public.cards
    FOR ALL
    TO authenticated
    USING (
        cardgroup_id IN (
            SELECT id
            FROM public.cardgroups
            WHERE owner_id = auth.uid()
        )
        OR public.is_admin(auth.uid())
    )
    WITH CHECK (
        cardgroup_id IN (
            SELECT id
            FROM public.cardgroups
            WHERE owner_id = auth.uid()
        )
        OR public.is_admin(auth.uid())
    );

CREATE POLICY swipe_records_select_own_or_admin
    ON public.swipe_records
    FOR SELECT
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY swipe_records_insert_own
    ON public.swipe_records
    FOR INSERT
    TO authenticated
    WITH CHECK (user_id = auth.uid());

CREATE POLICY roles_select_public
    ON public.roles
    FOR SELECT
    TO PUBLIC
    USING (true);

CREATE POLICY roles_insert_admin
    ON public.roles
    FOR INSERT
    TO authenticated
    WITH CHECK (public.is_admin(auth.uid()));

CREATE POLICY roles_update_admin
    ON public.roles
    FOR UPDATE
    TO authenticated
    USING (public.is_admin(auth.uid()))
    WITH CHECK (public.is_admin(auth.uid()));

CREATE POLICY roles_delete_admin
    ON public.roles
    FOR DELETE
    TO authenticated
    USING (public.is_admin(auth.uid()));

CREATE POLICY user_roles_select_self_or_admin
    ON public.user_roles
    FOR SELECT
    TO authenticated
    USING (user_id = auth.uid() OR public.is_admin(auth.uid()));

CREATE POLICY user_roles_insert_admin
    ON public.user_roles
    FOR INSERT
    TO authenticated
    WITH CHECK (public.is_admin(auth.uid()));

CREATE POLICY user_roles_update_admin
    ON public.user_roles
    FOR UPDATE
    TO authenticated
    USING (public.is_admin(auth.uid()))
    WITH CHECK (public.is_admin(auth.uid()));

CREATE POLICY user_roles_delete_admin
    ON public.user_roles
    FOR DELETE
    TO authenticated
    USING (public.is_admin(auth.uid()));

COMMIT;
