import { redirect } from "next/navigation";
import type { MyCardgroupsConnectionQuery as MyCardgroupsConnectionQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import CardgroupsClient from "./cardgroups-client";
import { CARDGROUPS_DEFAULT_VARS, MyCardgroupsConnectionQuery } from "./queries";

type CardgroupConnection = MyCardgroupsConnectionQueryType["myCardgroupsConnection"];

export default async function CardgroupsPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[cardgroups] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  let initialConnection: CardgroupConnection | null = null;
  try {
    const data = await gqlFetch(MyCardgroupsConnectionQuery, {
      variables: CARDGROUPS_DEFAULT_VARS,
      revalidate: 0,
    });
    initialConnection = data.myCardgroupsConnection;
  } catch (err) {
    // Structural parse per .claude/rules/frontend-rsc-error-handling.md §
    // "Structurally parse GraphQL extensions.code — never substring-match".
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    console.error(
      "[cardgroups] gqlFetch failed:",
      err instanceof Error ? err.name : "unknown",
      err instanceof Error ? err.message : String(err),
    );
    throw err;
  }

  return <CardgroupsClient initialConnection={initialConnection} />;
}
