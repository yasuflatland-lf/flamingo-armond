import { redirect } from "next/navigation";
import {
  CardsByCardgroupConnectionQuery,
  cardsDefaultVars,
} from "@/app/cardgroups/[id]/cards/queries";
import { CardgroupQuery } from "@/app/cardgroups/queries";
import type {
  CardgroupQuery as CardgroupQueryType,
  CardsByCardgroupConnectionQuery as CardsByCardgroupConnectionQueryType,
} from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { CardgroupManagementClient } from "./cardgroup-management-client";

type Props = {
  params: Promise<{ id: string }>;
};

export default async function EditCardgroupPage({ params }: Props) {
  const { id } = await params;

  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[cardgroups/:id/edit] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  let cardgroupData: CardgroupQueryType | null = null;
  let connectionData: CardsByCardgroupConnectionQueryType | null = null;

  try {
    [cardgroupData, connectionData] = await Promise.all([
      gqlFetch(CardgroupQuery, { variables: { id }, revalidate: 0 }),
      gqlFetch(CardsByCardgroupConnectionQuery, {
        variables: cardsDefaultVars(id),
        revalidate: 0,
      }),
    ]);
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) redirect("/cardgroups");
    console.error(
      "[cardgroups/:id/edit] gqlFetch failed:",
      err instanceof Error ? err.name : "unknown",
      err instanceof Error ? err.message : String(err),
    );
    throw err;
  }

  if (!cardgroupData?.cardgroup) redirect("/cardgroups");

  const cardgroup = cardgroupData.cardgroup;
  const initialEdges = connectionData?.cardsByCardgroupConnection.edges ?? [];
  const initialPageInfo = connectionData?.cardsByCardgroupConnection.pageInfo ?? {
    hasNextPage: false,
    hasPreviousPage: false,
    startCursor: null,
    endCursor: null,
  };
  const initialTotalCount = connectionData?.cardsByCardgroupConnection.totalCount ?? 0;

  return (
    <CardgroupManagementClient
      cardgroup={{ id: cardgroup.id, name: cardgroup.name }}
      initialEdges={initialEdges}
      initialPageInfo={initialPageInfo}
      initialTotalCount={initialTotalCount}
    />
  );
}
