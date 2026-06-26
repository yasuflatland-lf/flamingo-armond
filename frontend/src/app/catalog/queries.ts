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
 * ({@link CatalogListItem}), the onboarding chooser tile ({@link CatalogCard}),
 * and the merge-from-catalog sheet ({@link MergeFromCatalogSheet}).
 * `MasterCatalogQuery` (the /catalog list), `OnboardingStartQuery` (the
 * /onboarding/start chooser), and the merge sheet's inline `MasterCatalog` query
 * all spread this fragment, so all three queries share a single declared contract
 * instead of hand-mirrored node selections that can silently drift. Each consumer
 * unmasks it via `useFragment`.
 */
export const CatalogCardFieldsFragment = graphql(`
  fragment CatalogCardFields on MasterCardgroup {
    id
    name
    description
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

export const MergeMasterCardgroupMutation = graphql(`
  mutation MergeMasterCardgroup($input: MergeMasterCardgroupInput!) {
    mergeMasterCardgroup(input: $input) {
      __typename
      ... on MergeMasterCardgroupSuccess {
        cardgroup {
          id
          name
          updatedAt
        }
        addedCount
        updatedCount
      }
      ... on MasterNotFoundError {
        message
      }
    }
  }
`);

export const MergeMasterCardgroupPreviewQuery = graphql(`
  query MergeMasterCardgroupPreview($input: MergeMasterCardgroupInput!) {
    mergeMasterCardgroupPreview(input: $input) {
      __typename
      ... on MergeMasterCardgroupPreview {
        addedCount
        updatedCount
      }
      ... on MasterNotFoundError {
        message
      }
    }
  }
`);
