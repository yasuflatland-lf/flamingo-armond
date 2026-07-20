import type { Metadata } from "next";
import { headers } from "next/headers";
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
import { readAuthContext } from "@/lib/supabase/auth-status";
import { CardgroupManagementClient } from "./cardgroup-management-client";

// Per-user private route — must not be indexed.
export const metadata: Metadata = { robots: { index: false, follow: false } };

type Props = {
  params: Promise<{ id: string }>;
};

export default async function EditCardgroupPage({ params }: Props) {
  const { id } = await params;

  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

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
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    console.error("[cardgroups/:id/edit] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
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
