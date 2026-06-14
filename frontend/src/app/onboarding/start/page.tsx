import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import type { OnboardingStartQuery as OnboardingStartQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isUserOnboarded } from "@/lib/auth/onboarding";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { OnboardingStartClient } from "./onboarding-start-client";
import { OnboardingStartQuery } from "./queries";

export const metadata: Metadata = { title: "Get started" };

/**
 * Chooser step for a just-onboarded, deckless user. Reached from the HomePage
 * redirect chain and from OnboardingForm success. Self-guards the
 * not-onboarded direct-URL bypass and falls back to the create screen when the
 * catalog is empty. Bypasses AppShell (bare route), so it renders its own
 * <main> landmark.
 */
export default async function OnboardingStartPage() {
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

  let data: OnboardingStartQueryType;
  try {
    data = await gqlFetch(OnboardingStartQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    console.error(
      "[onboarding-start] gqlFetch failed:",
      err instanceof Error ? err.name : "unknown",
    );
    throw err;
  }

  if (!isUserOnboarded(data.me)) redirect("/onboarding");

  const catalog = data.masterCatalog;
  if (!catalog) {
    console.error("[onboarding-start] masterCatalog is null — partial response from backend");
    throw new Error("masterCatalog missing from onboarding-start data");
  }
  if (catalog.totalCount === 0) redirect("/cardgroups/new?welcome=1");

  const decks = catalog.edges.map((edge) => edge.node);

  return (
    <main className="p-8">
      <OnboardingStartClient decks={decks} />
    </main>
  );
}
