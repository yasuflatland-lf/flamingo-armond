import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { CardsNewBootstrapQuery as CardsNewBootstrapQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { CardsNewSkeleton } from "./_components/cards-new-skeleton";
import CardsNewClient from "./cards-new-client";
import { CardsNewBootstrapQuery } from "./queries";

interface CardsNewPageProps {
  searchParams: Promise<{ cardgroup?: string }>;
}

export default async function CardsNewPage({ searchParams }: CardsNewPageProps) {
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
    console.error("[cards-new] getUser() failed:", { name: authErr.name });
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  const { cardgroup: cardgroupParam } = await searchParams;

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">New card</h1>
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
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[cards-new] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  const myCardgroups = bootstrapData.myCardgroupsConnection?.edges?.map((e) => e.node) ?? [];
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
