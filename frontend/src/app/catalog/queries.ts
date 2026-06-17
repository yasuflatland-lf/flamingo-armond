import { graphql } from "@/generated";
import type { MasterCatalogQueryVariables } from "@/generated/graphql";

// Master catalog queries

/** Shared page size for masterCatalog — SSR seed and client must use the same value. */
const CATALOG_PAGE_SIZE = 20;

/**
 * Default variables for {@link MasterCatalogDocument}. Every read site — RSC
 * seed, client `useQuery`, and cache reads/writes — MUST use this object (or
 * spread from it) so Apollo's cache key is identical across all three.
 * Hard-coding `{ first: 20 }` in one place and `{ first: 20, search: null }` in
 * another silently splits the cache and makes the SSR seed dead code.
 *
 * See: docs/pagination/variables-shape-must-match.md
 */
export const CATALOG_DEFAULT_VARS: MasterCatalogQueryVariables = {
  first: CATALOG_PAGE_SIZE,
  search: null,
};

/**
 * The `MasterCardgroup` field set shared by the catalog list row
 * ({@link CatalogListItem}) and the onboarding chooser tile ({@link CatalogCard}).
 * Both `MasterCatalogQuery` (the /catalog list) and `OnboardingStartQuery` (the
 * /onboarding/start chooser) spread this fragment, so the two queries share a
 * single declared contract instead of two hand-mirrored node selections that
 * can silently drift. Each consumer unmasks it via `useFragment`.
 */
export const CatalogCardFieldsFragment = graphql(`
  fragment CatalogCardFields on MasterCardgroup {
    id
    name
    description
    language
    level
    category
    cardCount
  }
`);

export const MasterCatalogQuery = graphql(`
  query MasterCatalog(
    $first: Int
    $after: ID
    $last: Int
    $before: ID
    $search: String
    $orderBy: MasterCatalogOrderBy
    $orderDirection: SortOrder
  ) {
    masterCatalog(
      first: $first
      after: $after
      last: $last
      before: $before
      search: $search
      orderBy: $orderBy
      orderDirection: $orderDirection
    ) {
      edges {
        cursor
        node {
          id
          ...CatalogCardFields
        }
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
      totalCount
    }
  }
`);

// Master catalog mutations

export const ImportMasterCardgroupMutation = graphql(`
  mutation ImportMasterCardgroup($masterCardgroupId: ID!) {
    importMasterCardgroup(masterCardgroupId: $masterCardgroupId) {
      __typename
      ... on ImportMasterCardgroupSuccess {
        cardgroup {
          id
          name
          updatedAt
        }
      }
      ... on MasterNotFoundError {
        message
      }
    }
  }
`);
