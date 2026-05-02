import { redirect } from "next/navigation";
import type { CardsNewBootstrapQuery as CardsNewBootstrapQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
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

  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[cards-new] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  // --- Bootstrap data ---
  let bootstrapData: CardsNewBootstrapQueryType | null = null;
  try {
    bootstrapData = await gqlFetch(CardsNewBootstrapQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[cards-new] gqlFetch failed:", err);
    throw err;
  }

  if (!bootstrapData) {
    throw new Error("[cards-new] unreachable: gqlFetch resolved without data");
  }

  const myCardgroups = bootstrapData.myCardgroups;
  const lastViewedId = bootstrapData.me?.lastViewedCardgroup?.id ?? null;

  // --- Resolve cardgroup ---
  const { cardgroup: cardgroupParam } = await searchParams;

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
    <main className="mx-auto max-w-2xl p-8">
      <h1 className="mb-6 text-2xl font-semibold">New card</h1>
      <CardsNewClient
        initialCardgroupId={resolvedCardgroupId}
        forcePickerOpen={forcePickerOpen}
        myCardgroups={myCardgroups}
      />
    </main>
  );
}
