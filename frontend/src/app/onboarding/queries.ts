import { graphql } from "@/generated";

/**
 * Bootstrap query for the Onboarding RSC.
 *
 * Fetches the current user's id and displayName so the onboarding page
 * can greet the user by name and seed the profile form.
 *
 * Auth-sensitive (requires Authorization), so callers MUST pass `revalidate: 0`
 * to gqlFetch.
 */
export const OnboardingMeQuery = graphql(`
  query OnboardingMe {
    me {
      id
      displayName
    }
  }
`);
