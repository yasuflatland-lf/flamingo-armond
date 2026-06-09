import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { CardgroupQuery } from "@/app/cardgroups/queries";
import { MeWithLastViewedQuery } from "@/app/queries";
import type {
  CardgroupQuery as CardgroupQueryType,
  LearnNextDueCardsQuery as LearnNextDueCardsQueryType,
  MeWithLastViewedQuery as MeWithLastViewedQueryType,
} from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { LEARN_PAGE_LIMIT, LearnNextDueCardsQuery } from "../queries";
import { LearnAddCardSheet } from "./_components/learn-add-card-sheet";
import { LearnSkeleton } from "./_components/learn-skeleton";
import { LearnClient } from "./learn-client";

// Per-user learning session — must not be indexed.
export const metadata: Metadata = {
  title: "Learn",
  robots: { index: false, follow: false },
};

export default async function LearnPage({ params }: { params: Promise<{ cardgroupId: string }> }) {
  // Auth runs OUTSIDE the Suspense boundary so the redirect fires before any
  // streaming begins — Next.js cannot redirect mid-stream.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

  const { cardgroupId } = await params;

  return (
    <main className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col p-4">
        <Suspense fallback={<LearnSkeleton />}>
          <LearnContent cardgroupId={cardgroupId} />
        </Suspense>
      </div>
      {/* Mounted outside the Suspense boundary so the '+' add-card drawer works
          regardless of the queue state (including the AllCaughtUp empty state). */}
      <LearnAddCardSheet cardgroupId={cardgroupId} />
    </main>
  );
}

/**
 * Data-dependent subtree streamed inside the `<Suspense>` boundary. Auth has
 * already passed at this point; this component only loads the data needed to
 * hydrate `<LearnClient>` and handles the post-auth redirect cases (missing
 * cardgroup, GraphQL UNAUTHENTICATED).
 */
async function LearnContent({ cardgroupId }: { cardgroupId: string }) {
  let cardgroupData: CardgroupQueryType;
  let cardsData: LearnNextDueCardsQueryType;
  let meData: MeWithLastViewedQueryType;

  try {
    [cardgroupData, cardsData, meData] = await Promise.all([
      gqlFetch(CardgroupQuery, { variables: { id: cardgroupId }, revalidate: 0 }),
      gqlFetch(LearnNextDueCardsQuery, {
        variables: { cardgroupId, limit: LEARN_PAGE_LIMIT },
        revalidate: 0,
      }),
      gqlFetch(MeWithLastViewedQuery, { revalidate: 0 }),
    ]);
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[learn] gqlFetch batch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (!cardgroupData.cardgroup) redirect("/cardgroups");

  const cards = cardsData.learnNextDueCards;
  const lastViewedCardgroupId = meData.me?.lastViewedCardgroup?.id ?? null;

  return (
    <LearnClient
      cardgroupId={cardgroupId}
      initialCards={cards}
      lastViewedCardgroupId={lastViewedCardgroupId}
    />
  );
}
