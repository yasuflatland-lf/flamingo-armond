import { graphql } from "@/generated";
import type { AdminMasterCardsConnectionQueryVariables } from "@/generated/graphql";

/** Default page size for the admin master-cards connection. Must stay in sync between SSR seed and client useQuery/cache reads. */
export const MASTER_CARDS_PAGE_SIZE = 20;

export const AdminMasterCardsConnectionQuery = graphql(`
  query AdminMasterCardsConnection(
    $masterCardgroupId: ID!
    $first: Int
    $after: ID
    $search: String
  ) {
    adminMasterCardsConnection(
      masterCardgroupId: $masterCardgroupId
      first: $first
      after: $after
      search: $search
    ) {
      edges {
        cursor
        node {
          id
          masterCardgroupId
          front
          back
          position
          createdAt
          updatedAt
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
 * Default variables for {@link AdminMasterCardsConnectionDocument}, scoped by
 * the route's master deck id. Every read site — RSC seed, client `useQuery`,
 * mutation `update` callbacks — MUST use this factory (or spread its result) so
 * Apollo's cache key is identical across all three.
 * See: docs/pagination/variables-shape-must-match.md
 */
export function masterCardsDefaultVars(
  masterCardgroupId: string,
): AdminMasterCardsConnectionQueryVariables {
  return {
    masterCardgroupId,
    first: MASTER_CARDS_PAGE_SIZE,
    search: null,
  };
}

export const AdminCreateMasterCard = graphql(`
  mutation AdminCreateMasterCard($input: NewMasterCardInput!) {
    adminCreateMasterCard(input: $input) {
      __typename
      ... on CreateMasterCardSuccess {
        masterCard {
          id
          masterCardgroupId
          front
          back
          position
          createdAt
          updatedAt
        }
      }
      ... on MasterCardDuplicateFrontError {
        message
        existingCardId
        existingBack
      }
    }
  }
`);

export const AdminUpdateMasterCard = graphql(`
  mutation AdminUpdateMasterCard($id: ID!, $input: UpdateMasterCardInput!) {
    adminUpdateMasterCard(id: $id, input: $input) {
      __typename
      ... on UpdateMasterCardSuccess {
        masterCard {
          id
          masterCardgroupId
          front
          back
          position
          createdAt
          updatedAt
        }
      }
      ... on InputValidationError {
        field
        message
      }
    }
  }
`);

export const AdminDeleteMasterCard = graphql(`
  mutation AdminDeleteMasterCard($id: ID!) {
    adminDeleteMasterCard(id: $id)
  }
`);

export const AdminDeleteMasterCards = graphql(`
  mutation AdminDeleteMasterCards($ids: [ID!]!) {
    adminDeleteMasterCards(ids: $ids)
  }
`);

export const AdminImportMasterCards = graphql(`
  mutation AdminImportMasterCards($input: ImportMasterCardsInput!) {
    adminImportMasterCards(input: $input) {
      inserted
      updated
      errors {
        line
        message
        kind
      }
    }
  }
`);
