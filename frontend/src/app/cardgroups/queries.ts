import { graphql } from "@/generated";
import type { MyCardgroupsConnectionQueryVariables } from "@/generated/graphql";

// Cardgroup queries

/** Shared page size for myCardgroupsConnection — SSR seed and client must use the same value. */
export const CARDGROUPS_PAGE_SIZE = 20;

/**
 * Default variables for {@link MyCardgroupsConnectionDocument}. Every read
 * site — RSC seed, client `useQuery`, and cache reads/writes in mutation
 * `update` callbacks — MUST use this object (or spread from it) so Apollo's
 * cache key is identical across all three. Hard-coding `{ first: 20 }` in one
 * place and `{ first: 20, search: null }` in another silently splits the cache
 * and makes SSR seeds dead code.
 *
 * See: .claude/rules/pagination.md § "Variables shape MUST match between SSR
 * seed and client cache reads"
 */
export const CARDGROUPS_DEFAULT_VARS: MyCardgroupsConnectionQueryVariables = {
  first: CARDGROUPS_PAGE_SIZE,
  search: null,
};

/** @deprecated Use MyCardgroupsConnectionQuery */
export const MyCardgroupsQuery = graphql(`
  query MyCardgroups {
    myCardgroups {
      id
      name
      updatedAt
    }
  }
`);

export const MyCardgroupsConnectionQuery = graphql(`
  query MyCardgroupsConnection(
    $first: Int
    $after: ID
    $last: Int
    $before: ID
    $search: String
    $orderBy: CardgroupOrderBy
    $orderDirection: SortOrder
  ) {
    myCardgroupsConnection(
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
          name
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

export const CardgroupQuery = graphql(`
  query Cardgroup($id: ID!) {
    cardgroup(id: $id) {
      id
      name
      updatedAt
    }
  }
`);

// Cardgroup mutations

export const CreateCardgroupMutation = graphql(`
  mutation CreateCardgroup($input: NewCardgroupInput!) {
    createCardgroup(input: $input) {
      cardgroup {
        id
        name
        updatedAt
      }
    }
  }
`);

export const UpdateCardgroupMutation = graphql(`
  mutation UpdateCardgroup($id: ID!, $input: UpdateCardgroupInput!) {
    updateCardgroup(id: $id, input: $input) {
      cardgroup {
        id
        name
        updatedAt
      }
    }
  }
`);

export const DeleteCardgroupMutation = graphql(`
  mutation DeleteCardgroup($id: ID!) {
    deleteCardgroup(id: $id)
  }
`);

// Card queries

export const CardsByCardgroupQuery = graphql(`
  query CardsByCardgroup($cardgroupId: ID!) {
    cardsByCardgroup(cardgroupId: $cardgroupId) {
      id
      front
      back
      due
      state
      cardgroupId
    }
  }
`);

// Card mutations

export const CreateCardMutation = graphql(`
  mutation CreateCard($input: NewCardInput!) {
    createCard(input: $input) {
      card {
        id
        front
        back
        due
        state
        cardgroupId
      }
    }
  }
`);

export const UpdateCardMutation = graphql(`
  mutation UpdateCard($id: ID!, $input: UpdateCardInput!) {
    updateCard(id: $id, input: $input) {
      card {
        id
        front
        back
        due
        state
        cardgroupId
      }
    }
  }
`);

export const DeleteCardMutation = graphql(`
  mutation DeleteCard($id: ID!) {
    deleteCard(id: $id)
  }
`);

export const DeleteCardsMutation = graphql(`
  mutation DeleteCards($ids: [ID!]!) {
    deleteCards(ids: $ids)
  }
`);
