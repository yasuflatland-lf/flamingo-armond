import { redirect } from "next/navigation";
import type { OnboardingMeQuery as OnboardingMeQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isUserOnboarded } from "@/lib/auth/onboarding";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { OnboardingForm } from "./onboarding-form";
import { OnboardingMeQuery } from "./queries";

export default async function OnboardingPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();

  // AuthSessionMissingError (anonymous request) and stale-session errors are ignorable;
  // they fall through to the redirect below. All other auth errors are rethrown.
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
    console.error(
      "[onboarding] gqlFetch failed:",
      err instanceof Error ? err.name : "unknown",
      err instanceof Error ? err.message : String(err),
    );
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
