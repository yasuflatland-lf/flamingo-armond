import { graphql } from "@/generated";

// Auth-sensitive — callers MUST pass `revalidate: 0` to gqlFetch.
export const OnboardingMeQuery = graphql(`
  query OnboardingMe {
    me {
      id
      displayName
    }
  }
`);
