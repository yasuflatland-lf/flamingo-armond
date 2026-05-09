import { graphql } from "@/generated";

/**
 * Bootstrap query for the Onboarding RSC.
 *
 * Fetches displayName so the page can redirect already-onboarded users back to /.
 * Auth-sensitive — callers MUST pass `revalidate: 0` to gqlFetch.
 */
export const OnboardingMeQuery = graphql(`
  query OnboardingMe {
    me {
      id
      displayName
    }
  }
`);
