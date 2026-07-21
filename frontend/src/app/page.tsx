import { redirect } from "next/navigation";
import { MeWithLastViewedQuery } from "@/app/queries";
import type { MeWithLastViewedQuery as MeWithLastViewedQueryType } from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isUserOnboarded } from "@/lib/auth/onboarding";
import { requireAuthenticated } from "@/lib/supabase/auth-status";

// Root redirect — see docs/frontend/routing-topology.md.
export default async function HomePage() {
  await requireAuthenticated("/login");

  let data: MeWithLastViewedQueryType;
  try {
    data = await gqlFetch(MeWithLastViewedQuery, { revalidate: 0 });
  } catch (err) {
    redirectIfAuthError(err, "/login");
    console.error("[home] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (!isUserOnboarded(data.me)) redirect("/onboarding");

  const lastViewedId = data.me?.lastViewedCardgroup?.id ?? null;
  if (lastViewedId) {
    redirect(`/learn/${lastViewedId}`);
  }

  if (!data.myCardgroupsConnection) {
    console.error("[home] myCardgroupsConnection is null — partial response from backend");
    throw new Error("myCardgroupsConnection missing from root redirect data");
  }
  if (data.myCardgroupsConnection.totalCount > 0) {
    redirect("/cardgroups");
  }

  redirect("/onboarding/start");
}
