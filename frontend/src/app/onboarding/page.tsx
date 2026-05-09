import { redirect } from "next/navigation";
import type { OnboardingMeQuery as OnboardingMeQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isUserOnboarded } from "@/lib/auth/onboarding";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { OnboardingMeQuery } from "./queries";
import { OnboardingForm } from "./onboarding-form";

export default async function OnboardingPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();

  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[onboarding] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  let data: OnboardingMeQueryType;
  try {
    data = await gqlFetch(OnboardingMeQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[onboarding] gqlFetch failed:", err);
    throw err;
  }

  // Already-onboarded users should not re-enter the onboarding flow.
  if (isUserOnboarded(data.me)) redirect("/");

  // This page bypasses AppShell, so it renders its own <main> landmark.
  return (
    <main className="p-8">
      <OnboardingForm />
    </main>
  );
}
