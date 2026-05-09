import { redirect } from "next/navigation";
import { AdminRoleQuery } from "@/app/admin/roles/queries";
import type { AdminRoleQuery as AdminRoleQueryType } from "@/generated/graphql";
import {
  isForbiddenGraphQLError,
  isUnauthenticatedGraphQLError,
} from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { EditRoleClient, type RoleForEdit } from "./edit-role-client";

type Props = {
  params: Promise<{ id: string }>;
};

/**
 * RSC for editing an existing role.
 *
 * admin/layout.tsx is the source-of-truth admin gate; the per-page getUser
 * here is defense-in-depth. UNAUTHENTICATED and FORBIDDEN are routed
 * separately:
 *   - UNAUTHENTICATED → /login (session expired or never present).
 *   - FORBIDDEN → /admin/roles (the admin layout already let this caller in,
 *     so a FORBIDDEN here means a role lost its admin grant mid-session;
 *     bouncing to the listing avoids a double-redirect through /login).
 *
 * Per .claude/rules/frontend-rsc-error-handling.md § "Structurally parse
 * GraphQL extensions.code", the codes are matched via the structural helpers
 * — never by substring on err.message.
 */
export default async function EditRolePage({ params }: Props) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[admin/roles/:id/edit] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  const { id } = await params;

  let data: AdminRoleQueryType | null = null;
  try {
    data = await gqlFetch(AdminRoleQuery, { variables: { id }, revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    if (isForbiddenGraphQLError(err)) redirect("/admin/roles");
    console.error(
      "[admin/roles/:id/edit] gqlFetch failed:",
      err instanceof Error ? err.name : "unknown",
      err instanceof Error ? err.message : String(err),
    );
    throw err;
  }

  if (!data?.role) redirect("/admin/roles");

  const role = data.role as unknown as RoleForEdit;
  return <EditRoleClient role={role} />;
}
