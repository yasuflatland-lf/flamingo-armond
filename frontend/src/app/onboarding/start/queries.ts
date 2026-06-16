import { graphql } from "@/generated";

// Auth-sensitive — callers MUST pass `revalidate: 0` to gqlFetch.
//
// `me { displayName }` drives the not-onboarded self-guard; `masterCatalog`
// supplies the chooser gallery (first 20, no pagination in onboarding) and the
// `totalCount === 0` empty-catalog fallback.
export const OnboardingStartQuery = graphql(`
  query OnboardingStart {
    me {
      id
      displayName
    }
    masterCatalog(first: 20) {
      edges {
        cursor
        node {
          id
          ...CatalogCardFields
        }
      }
      totalCount
    }
  }
`);

// The `{ id, name, updatedAt }` node projection matches `prependMyCardgroupEdge`'s
// `MyCardgroupNode` so each returned cardgroup can be prepended to the
// `myCardgroupsConnection` cache without a refetch.
export const SeedDefaultStartersMutation = graphql(`
  mutation SeedDefaultStarterCardgroups {
    seedDefaultStarterCardgroups {
      cardgroups {
        id
        name
        updatedAt
      }
    }
  }
`);
