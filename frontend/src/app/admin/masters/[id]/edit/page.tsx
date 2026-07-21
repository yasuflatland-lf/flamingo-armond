import type { Metadata } from "next";
import { notFound } from "next/navigation";
import type {
  AdminMasterCardsConnectionQuery as AdminMasterCardsConnectionQueryType,
  AdminMasterQuery,
} from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { AdminMasterCardsConnectionQuery, masterCardsDefaultVars } from "./cards/queries";
import { MasterManagementClient } from "./master-management-client";
import { AdminMasterQueryDocument } from "./queries";

// Admin-only route — must not be indexed.
export const metadata: Metadata = { robots: { index: false, follow: false } };

type Props = { params: Promise<{ id: string }> };

export default async function EditMasterPage({ params }: Props) {
  const { id } = await params;

  // Defense-in-depth under the admin layout: redirect to / for stale/anonymous.
  await requireAuthenticated("/");

  let deckData: AdminMasterQuery | null = null;
  let connectionData: AdminMasterCardsConnectionQueryType | null = null;
  try {
    [deckData, connectionData] = await Promise.all([
      gqlFetch(AdminMasterQueryDocument, { variables: { id }, revalidate: 0 }),
      gqlFetch(AdminMasterCardsConnectionQuery, {
        variables: masterCardsDefaultVars(id),
        revalidate: 0,
      }),
    ]);
  } catch (err) {
    redirectIfAuthError(err, "/", { forbidden: true });
    console.error("[admin/masters/:id/edit] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (!deckData?.adminMaster) notFound();

  const conn = connectionData?.adminMasterCardsConnection;
  const initialEdges = conn?.edges ?? [];
  const initialPageInfo = conn?.pageInfo ?? {
    __typename: "PageInfo" as const,
    hasNextPage: false,
    hasPreviousPage: false,
    startCursor: null,
    endCursor: null,
  };
  const initialTotalCount = conn?.totalCount ?? 0;

  return (
    <MasterManagementClient
      master={deckData.adminMaster}
      initialEdges={initialEdges}
      initialPageInfo={initialPageInfo}
      initialTotalCount={initialTotalCount}
    />
  );
}
