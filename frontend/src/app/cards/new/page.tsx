import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import type { CardsNewBootstrapQuery as CardsNewBootstrapQueryType } from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { CardsNewSkeleton } from "./_components/cards-new-skeleton";
import CardsNewClient from "./cards-new-client";
import { CardsNewBootstrapQuery } from "./queries";

export const metadata: Metadata = { title: "New card" };

interface CardsNewPageProps {
  searchParams: Promise<{ cardgroup?: string }>;
}

export default async function CardsNewPage({ searchParams }: CardsNewPageProps) {
  // Auth check runs OUTSIDE the Suspense boundary so an unauthenticated request
  // redirects to /login before any streaming starts. The middleware forwards
  // identity via x-auth-status; a missing or malformed header degrades to
  // "anonymous" so a dropped header never leaks an authenticated view.
  await requireAuthenticated("/login");

  const t = await getTranslations("Cards");
  const { cardgroup: cardgroupParam } = await searchParams;

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold" data-testid="cards-new-page-heading">
        {t("newCardTitle")}
      </h1>
      <Suspense fallback={<CardsNewSkeleton />}>
        <CardsNewContent cardgroupParam={cardgroupParam} />
      </Suspense>
    </main>
  );
}

/**
 * Inner async server component that performs the GraphQL fetch. Extracted from
 * `CardsNewPage` so the route shell can stream while the bootstrap query resolves.
 *
 * Exported as a named export (not the default) so the RSC test can invoke it
 * directly without going through React's Suspense renderer; the outer page
 * test verifies the Suspense boundary and skeleton fallback separately.
 */
export async function CardsNewContent({ cardgroupParam }: { cardgroupParam: string | undefined }) {
  let bootstrapData: CardsNewBootstrapQueryType;
  try {
    bootstrapData = await gqlFetch(CardsNewBootstrapQuery, { revalidate: 0 });
  } catch (err) {
    redirectIfAuthError(err, "/login");
    console.error("[cards-new] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  const conn = bootstrapData.myCardgroupsConnection;
  if (!conn) {
    console.error("[cards-new] myCardgroupsConnection is null — partial response from backend");
    throw new Error("myCardgroupsConnection missing from bootstrap data");
  }
  const myCardgroups = conn.edges.map((e) => e.node);
  const lastViewedId = bootstrapData.me?.lastViewedCardgroup?.id ?? null;

  // Build a quick-lookup set for ownership checks.
  const ownedIds = new Set(myCardgroups.map((cg) => cg.id));

  let resolvedCardgroupId: string | null;
  let forcePickerOpen: boolean;

  if (cardgroupParam && ownedIds.has(cardgroupParam)) {
    // Priority 1: valid ?cardgroup= param owned by the user.
    resolvedCardgroupId = cardgroupParam;
    forcePickerOpen = false;
  } else if (lastViewedId && ownedIds.has(lastViewedId)) {
    // Priority 2: me.lastViewedCardgroup exists and is owned by the user.
    resolvedCardgroupId = lastViewedId;
    forcePickerOpen = false;
  } else if (myCardgroups.length > 0) {
    // Priority 3: user has cardgroups but none is pre-selected — force picker open.
    resolvedCardgroupId = null;
    forcePickerOpen = true;
  } else {
    // Priority 4: no cardgroups at all — redirect to onboarding.
    redirect("/cardgroups/new?welcome=1");
  }

  return (
    <CardsNewClient
      initialCardgroupId={resolvedCardgroupId}
      forcePickerOpen={forcePickerOpen}
      myCardgroups={myCardgroups}
    />
  );
}
