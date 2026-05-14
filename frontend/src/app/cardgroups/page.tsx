import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { MyCardgroupsConnectionQuery as MyCardgroupsConnectionQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { CardgroupsSkeleton } from "./_components/cardgroups-skeleton";
import CardgroupsClient from "./cardgroups-client";
import { CARDGROUPS_DEFAULT_VARS, MyCardgroupsConnectionQuery } from "./queries";

type CardgroupConnection = MyCardgroupsConnectionQueryType["myCardgroupsConnection"];

export default async function CardgroupsPage() {
  // Auth check runs OUTSIDE the Suspense boundary so a stale session redirects
  // to /login before any streaming starts. If the redirect ran from within the
  // suspended subtree, the skeleton would flash before the navigation.
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

  return (
    <Suspense fallback={<CardgroupsSkeleton />}>
      <CardgroupsContent />
    </Suspense>
  );
}

/**
 * Inner async server component that performs the GraphQL fetch. Extracted from
 * `CardgroupsPage` so the route shell can stream while this query resolves.
 *
 * Exported as a named export (not the default) so the RSC test can invoke it
 * directly without going through React's Suspense renderer; the outer page
 * test verifies the Suspense boundary and skeleton fallback separately.
 */
export async function CardgroupsContent() {
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
