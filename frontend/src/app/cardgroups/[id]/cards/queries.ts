import { graphql } from "@/generated";

/** Default page size for the cards-by-cardgroup connection. Must stay in sync between SSR seed and client useQuery/cache reads. */
export const CARDS_PAGE_SIZE = 20;

// Forward-only Relay connection query for paginating cards within a cardgroup.
export const CardsByCardgroupConnectionQuery = graphql(`
  query CardsByCardgroupConnection(
    $cardgroupId: ID!
    $first: Int
    $after: ID
  ) {
    cardsByCardgroupConnection(cardgroupId: $cardgroupId, first: $first, after: $after) {
      edges {
        cursor
        node {
          id
          front
          back
          due
          state
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
