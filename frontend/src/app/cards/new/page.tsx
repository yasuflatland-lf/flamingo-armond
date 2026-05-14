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
  // --- Auth ---
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();

  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  // Auth runs OUTSIDE the Suspense boundary so the redirect fires before any
  // streaming begins — Next.js cannot redirect mid-stream.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[cards-new] getUser() failed:", { name: authErr.name });
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  // The ?cardgroup= param is part of the URL contract — resolve it before the
  // Suspense boundary so `CardsNewContent` receives a plain string and the
  // suspended subtree does not need to await a Promise just to read it.
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
 * Data-dependent subtree streamed inside the `<Suspense>` boundary. Auth has
 * already passed at this point; this component only loads the bootstrap data
 * needed to hydrate `<CardsNewClient>` and handles the post-auth redirect
 * cases (GraphQL UNAUTHENTICATED, no cardgroups → onboarding).
 */
async function CardsNewContent({ cardgroupParam }: { cardgroupParam: string | undefined }) {
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

  const myCardgroups = bootstrapData.myCardgroups;
  const lastViewedId = bootstrapData.me?.lastViewedCardgroup?.id ?? null;

  // --- Resolve cardgroup ---
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
    // Priority 3: user has cardgroups but none is pre-selected — render the chip
    // in an undetermined state and force the picker open so the user can choose.
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
