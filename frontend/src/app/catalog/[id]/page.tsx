import type { Metadata } from "next";
import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { Suspense } from "react";
import type {
  CatalogMasterCardsConnectionQuery as CatalogMasterCardsConnectionQueryType,
  CatalogMasterDeckQuery as CatalogMasterDeckQueryType,
} from "@/generated/graphql";
import {
  isBadUserInputGraphQLError,
  isUnauthenticatedGraphQLError,
} from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { CatalogDeckSkeleton } from "./_components/catalog-deck-skeleton";
import CatalogDeckClient from "./catalog-deck-client";
import {
  CatalogMasterCardsConnectionQuery,
  CatalogMasterDeckQuery,
  catalogCardsDefaultVars,
} from "./queries";

export const metadata: Metadata = { title: "Deck" };

type Props = { params: Promise<{ id: string }> };

export default async function CatalogDeckPage({ params }: Props) {
  // Auth check runs OUTSIDE the Suspense boundary so an unauthenticated request
  // redirects to /login before any streaming starts. If the redirect ran from
  // within the suspended subtree, the skeleton would flash before the navigation.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

  const { id } = await params;

  return (
    <Suspense fallback={<CatalogDeckSkeleton />}>
      <CatalogDeckContent id={id} />
    </Suspense>
  );
}

/**
 * Inner async server component that performs the two parallel GraphQL fetches.
 * Extracted from `CatalogDeckPage` so the route shell can stream while the
 * queries resolve, and exported as a named export so the RSC test can invoke it
 * directly without going through React's Suspense renderer.
 *
 * Failure mapping:
 *  - `masterCardgroup == null` (unknown or DRAFT id, non-disclosure gate) → notFound().
 *  - `masterCardsConnection` rejects with BAD_USER_INPUT (same non-disclosure
 *    gate, expressed as an error on `masterCardgroupId`) → notFound().
 *  - UNAUTHENTICATED → redirect("/login").
 */
export async function CatalogDeckContent({ id }: { id: string }) {
  let deckData: CatalogMasterDeckQueryType;
  let cardsData: CatalogMasterCardsConnectionQueryType;
  try {
    [deckData, cardsData] = await Promise.all([
      gqlFetch(CatalogMasterDeckQuery, { variables: { id }, revalidate: 0 }),
      gqlFetch(CatalogMasterCardsConnectionQuery, {
        variables: catalogCardsDefaultVars(id),
        revalidate: 0,
      }),
    ]);
  } catch (err) {
    // Structural parse per .claude/rules/frontend-rsc-error-handling.md §
    // "Structurally parse GraphQL extensions.code — never substring-match".
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    // A DRAFT or unknown deck rejects the cards query as BAD_USER_INPUT on
    // `masterCardgroupId` (draft existence is never revealed) → render the 404.
    if (isBadUserInputGraphQLError(err)) notFound();
    console.error("[catalog/:id] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  const deck = deckData.masterCardgroup;
  if (deck == null) notFound();

  const connection = cardsData.masterCardsConnection;
  // Guard the partial-response null bubble: the schema declares
  // `masterCardsConnection: MasterCardConnection!`, but a partial response
  // (GraphQL over HTTP §5.2) delivers it as null while codegen types it
  // non-null. `gqlFetch` returns that data with only a console.warn, so without
  // this guard the dereference below would crash inside Suspense with no
  // field-level signal. Mirrors `app/cardgroups/page.tsx` and `app/catalog/page.tsx`.
  if (connection == null) {
    console.error("[catalog/:id] masterCardsConnection is null — partial response from backend");
    throw new Error("masterCardsConnection missing from catalog deck data");
  }

  return (
    <CatalogDeckClient
      id={id}
      initialDeck={deck}
      initialEdges={connection.edges}
      initialPageInfo={connection.pageInfo}
      initialTotalCount={connection.totalCount}
    />
  );
}
