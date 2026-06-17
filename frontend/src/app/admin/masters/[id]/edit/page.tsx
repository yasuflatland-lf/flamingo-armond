import type { Metadata } from "next";
import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import type { AdminMasterQuery } from "@/generated/graphql";
import { AdminMasterDocument } from "@/generated/graphql";
import {
  isForbiddenGraphQLError,
  isUnauthenticatedGraphQLError,
} from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { MasterManagementClient } from "./master-management-client";

// Admin-only route — must not be indexed.
export const metadata: Metadata = { robots: { index: false, follow: false } };

type Props = { params: Promise<{ id: string }> };

export default async function EditMasterPage({ params }: Props) {
  const { id } = await params;

  // Defense-in-depth under the admin layout: redirect to / for stale/anonymous.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/");

  let data: AdminMasterQuery | null = null;
  try {
    data = await gqlFetch(AdminMasterDocument, { variables: { id }, revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err) || isForbiddenGraphQLError(err)) redirect("/");
    console.error("[admin/masters/:id/edit] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (!data?.adminMaster) notFound();

  return <MasterManagementClient master={data.adminMaster} />;
}
