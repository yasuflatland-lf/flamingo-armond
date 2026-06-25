import { graphql } from "@/generated";
import type { CatalogMasterCardsConnectionQueryVariables } from "@/generated/graphql";

// Public catalog deck-detail queries (read-only). These mirror the two GraphQL
// fetches behind the cardgroup edit screen — the cards connection
// (`cardsByCardgroupConnection`, in app/cardgroups/[id]/cards/queries.ts) and
// the deck metadata (`cardgroup`, in app/cardgroups/queries.ts) — but read the
// public, master-deck counterparts: `masterCardsConnection` (paginated cards in
// a PUBLISHED master deck) and `masterCardgroup` (a single published deck).

/** Shared page size for the catalog deck card list — SSR seed and client must use the same value. */
const CATALOG_CARDS_PAGE_SIZE = 20;

export const CatalogMasterCardsConnectionQuery = graphql(`
  query CatalogMasterCardsConnection(
    $masterCardgroupId: ID!
    $first: Int
    $after: ID
    $search: String
  ) {
    masterCardsConnection(
      masterCardgroupId: $masterCardgroupId
      first: $first
      after: $after
      search: $search
    ) {
      edges {
        cursor
        node {
          id
          front
          back
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

export const CatalogMasterDeckQuery = graphql(`
  query CatalogMasterDeck($id: ID!) {
    masterCardgroup(id: $id) {
      id
      name
      description
    }
  }
`);

/**
 * Default variables for {@link CatalogMasterCardsConnectionDocument}, scoped by
 * the route's `masterCardgroupId`. Both call sites — the RSC seed and the client
 * `useQuery` — MUST use this factory (or spread its result) so Apollo's cache key
 * is identical across both. (The deck-detail screen is read-only, so there are no
 * cache-write callbacks on this document.) Hard-coding
 * `{ masterCardgroupId, first: 20 }` in one place and
 * `{ masterCardgroupId, first: 20, search: null }` in another silently splits
 * the cache and makes the SSR seed dead code.
 *
 * `masterCardgroupId` is dynamic per-route, so the export is a factory rather
 * than a static object (mirrors `cardsDefaultVars` in
 * `app/cardgroups/[id]/cards/queries.ts`).
 *
 * See: docs/pagination/variables-shape-must-match.md
 */
export function catalogCardsDefaultVars(
  masterCardgroupId: string,
): CatalogMasterCardsConnectionQueryVariables {
  return {
    masterCardgroupId,
    first: CATALOG_CARDS_PAGE_SIZE,
    search: null,
  };
}
