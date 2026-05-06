import { redirect } from "next/navigation";
import type { AdminUsersQuery as AdminUsersQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { AdminUsersClient } from "./AdminUsersClient";
import { ADMIN_USERS_DEFAULT_VARS, AdminUsersQuery } from "./queries";

type UsersConnection = AdminUsersQueryType["users"];

export default async function AdminUsersPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[admin/users] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/");

  // Seed the default-view connection so the first useQuery pass on the client
  // is a cache hit. Non-default URLs (search / roleId / page > 0) miss the
  // seed and fall through to a client-side fetch.
  let initialConnection: UsersConnection | null = null;
  try {
    const data = await gqlFetch(AdminUsersQuery, {
      variables: ADMIN_USERS_DEFAULT_VARS,
      revalidate: 0,
    });
    initialConnection = data.users;
  } catch (err) {
    // Structurally parse per .claude/rules/frontend-rsc-error-handling.md §
    // "Structurally parse GraphQL extensions.code".
    if (isUnauthenticatedGraphQLError(err)) redirect("/");
    console.error(
      "[admin/users] gqlFetch failed:",
      err instanceof Error ? err.name : "unknown",
      err instanceof Error ? err.message : String(err),
    );
    // Non-auth failures (FORBIDDEN, INTERNAL, network) fall through with a
    // null seed. The client renders its query-error banner with a Retry.
  }

  return <AdminUsersClient initialConnection={initialConnection} />;
}
