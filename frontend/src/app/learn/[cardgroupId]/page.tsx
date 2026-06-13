import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { LearnNextDueCardsQuery as LearnNextDueCardsQueryType } from "@/generated/graphql";
import {
  isBadUserInputGraphQLError,
  isUnauthenticatedGraphQLError,
} from "@/lib/apollo/graphql-errors";
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
 * already passed at this point.
 *
 * The LCP element is the card text, which depends ONLY on `LearnNextDueCards`.
 * That query is therefore the single fetch on the LCP critical path — the
 * cardgroup-existence check and the last-viewed sync are deliberately NOT
 * fetched here, so they cannot hold the card paint hostage (see the learn-page
 * LCP investigation). Both concerns are recovered without an extra blocking
 * fetch: `LearnNextDueCards` already authorizes the cardgroup in the usecase
 * layer (`authorizeCardgroupForLearn`), so a missing cardgroup surfaces as a
 * `BAD_USER_INPUT` rejection (-> `/cardgroups`, mirroring the prior
 * CardgroupQuery guard) and a non-owned one as `UNAUTHENTICATED` (-> `/login`);
 * the last-viewed sync runs client-side in `<LearnClient>`.
 */
async function LearnContent({ cardgroupId }: { cardgroupId: string }) {
  let cardsData: LearnNextDueCardsQueryType;

  try {
    cardsData = await gqlFetch(LearnNextDueCardsQuery, {
      variables: { cardgroupId, limit: LEARN_PAGE_LIMIT },
      revalidate: 0,
    });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    // `LearnNextDueCards` only returns BAD_USER_INPUT for a missing cardgroup
    // (usecase `authorizeCardgroupForLearn` -> `NewValidationError("cardgroupId")`),
    // so this branch is the surviving form of the old cardgroup-existence guard.
    if (isBadUserInputGraphQLError(err)) {
      redirect("/cardgroups");
    }
    console.error("[learn] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (cardsData.me == null) {
    console.warn("[learn] me.learnDisplayMode unavailable; defaulting to FLIP_TO_REVEAL");
  }

  return (
    <LearnClient
      cardgroupId={cardgroupId}
      initialCards={cardsData.learnNextDueCards}
      displayMode={cardsData.me?.learnDisplayMode ?? "FLIP_TO_REVEAL"}
    />
  );
}
