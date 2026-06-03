import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { MyCardgroupsConnectionQuery as MyCardgroupsConnectionQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { CardgroupsSkeleton } from "./_components/cardgroups-skeleton";
import CardgroupsClient from "./cardgroups-client";
import { CARDGROUPS_DEFAULT_VARS, MyCardgroupsConnectionQuery } from "./queries";

export default async function CardgroupsPage() {
  // Auth check runs OUTSIDE the Suspense boundary so a stale session redirects
  // to /login before any streaming starts. If the redirect ran from within the
  // suspended subtree, the skeleton would flash before the navigation.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

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
  let data: MyCardgroupsConnectionQueryType;
  try {
    data = await gqlFetch(MyCardgroupsConnectionQuery, {
      variables: CARDGROUPS_DEFAULT_VARS,
      revalidate: 0,
    });
  } catch (err) {
    // Structural parse per .claude/rules/frontend-rsc-error-handling.md §
    // "Structurally parse GraphQL extensions.code — never substring-match".
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    console.error("[cardgroups] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  const initialConnection = data.myCardgroupsConnection;
  if (!initialConnection) {
    console.error("[cardgroups] myCardgroupsConnection is null — partial response from backend");
    throw new Error("myCardgroupsConnection missing from cardgroups data");
  }

  return <CardgroupsClient initialConnection={initialConnection} />;
}
