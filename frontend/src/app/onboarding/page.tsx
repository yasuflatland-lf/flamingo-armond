import { redirect } from "next/navigation";
import type { OnboardingMeQuery as OnboardingMeQueryType } from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isUserOnboarded } from "@/lib/auth/onboarding";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { OnboardingForm } from "./onboarding-form";
import { OnboardingMeQuery } from "./queries";

export default async function OnboardingPage() {
  await requireAuthenticated("/login");

  let data: OnboardingMeQueryType;
  try {
    data = await gqlFetch(OnboardingMeQuery, { revalidate: 0 });
  } catch (err) {
    redirectIfAuthError(err, "/login");
    console.error("[onboarding] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  if (isUserOnboarded(data.me)) redirect("/");

  // This page bypasses AppShell, so it renders its own <main> landmark. The
  // OnboardingShell owns the centered hero layout, so <main> carries no padding.
  return (
    <main>
      <OnboardingForm />
    </main>
  );
}
