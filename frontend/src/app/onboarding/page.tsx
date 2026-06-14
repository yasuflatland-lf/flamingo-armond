import { headers } from "next/headers";
import { redirect } from "next/navigation";
import type { OnboardingMeQuery as OnboardingMeQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isUserOnboarded } from "@/lib/auth/onboarding";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { OnboardingForm } from "./onboarding-form";
import { OnboardingMeQuery } from "./queries";

export default async function OnboardingPage() {
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

  let data: OnboardingMeQueryType;
  try {
    data = await gqlFetch(OnboardingMeQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[onboarding] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (isUserOnboarded(data.me)) redirect("/");

  // This page bypasses AppShell, so it renders its own <main> landmark.
  return (
    <main className="p-8">
      <OnboardingForm />
    </main>
  );
}
