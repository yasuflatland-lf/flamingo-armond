import { graphql } from "@/generated";
import type { CardsByCardgroupConnectionQueryVariables } from "@/generated/graphql";

/** Default page size for the cards-by-cardgroup connection. Must stay in sync between SSR seed and client useQuery/cache reads. */
export const CARDS_PAGE_SIZE = 20;

export const CardsByCardgroupConnectionQuery = graphql(`
  query CardsByCardgroupConnection(
    $cardgroupId: ID!
    $first: Int
    $after: ID
    $search: String
  ) {
    cardsByCardgroupConnection(
      cardgroupId: $cardgroupId
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
          userCardState {
            due
            state
          }
          cardgroupId
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

/**
 * Default variables for {@link CardsByCardgroupConnectionDocument}, scoped by
 * the route's `cardgroupId`. Every read site — RSC seed, client `useQuery`,
 * mutation `update` callbacks — MUST use this factory (or spread its result)
 * so Apollo's cache key is identical across all three. Hard-coding
 * `{ cardgroupId, first: 20 }` in one place and
 * `{ cardgroupId, first: 20, search: null }` in another silently splits the
 * cache and makes SSR seeds dead code.
 *
 * `cardgroupId` is dynamic per-route, so the export is a factory rather than
 * a static object (the cardgroups-list precedent in `app/cardgroups/queries.ts`
 * has no parent scope).
 *
 * See: docs/pagination/variables-shape-must-match.md
 */
export function cardsDefaultVars(cardgroupId: string): CardsByCardgroupConnectionQueryVariables {
  return {
    cardgroupId,
    first: CARDS_PAGE_SIZE,
    search: null,
  };
}
