import { graphql } from "@/generated";

/** Default page size for the admin masters connection. Keep in sync with cache reads. */
export const ADMIN_MASTERS_PAGE_SIZE = 20;

/**
 * Base variables for the admin masters connection (everything except `search`).
 * Single source of truth shared by the client's useConnectionPagination query and
 * the create-cache write in useMasterMutations — they MUST match exactly or the
 * prepended edge is written under a different cache key and never appears.
 */
export const ADMIN_MASTERS_BASE_VARS = {
  first: ADMIN_MASTERS_PAGE_SIZE,
  orderBy: "SORT_ORDER" as const,
  orderDirection: "ASC" as const,
};

export const AdminMastersQuery = graphql(`
  query AdminMasters(
    $first: Int
    $after: ID
    $last: Int
    $before: ID
    $search: String
    $orderBy: MasterCatalogOrderBy
    $orderDirection: SortOrder
  ) {
    adminMasters(
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
          description
          version
          status
          isDefaultStarter
          sortOrder
          cardCount
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

export const AdminCreateMasterMutation = graphql(`
  mutation AdminCreateMaster($input: CreateMasterCardgroupInput!) {
    adminCreateMasterCardgroup(input: $input) {
      __typename
      ... on CreateMasterCardgroupSuccess {
        master {
          id
          name
          description
          version
          status
          isDefaultStarter
          sortOrder
          cardCount
        }
      }
      ... on InputValidationError {
        field
        message
      }
    }
  }
`);

export const AdminUpdateMasterMutation = graphql(`
  mutation AdminUpdateMaster($id: ID!, $input: UpdateMasterCardgroupInput!) {
    adminUpdateMasterCardgroup(id: $id, input: $input) {
      __typename
      ... on UpdateMasterCardgroupSuccess {
        master {
          id
          name
          description
          version
          status
          isDefaultStarter
          sortOrder
          cardCount
        }
      }
      ... on InputValidationError {
        field
        message
      }
    }
  }
`);

export const AdminPublishMasterMutation = graphql(`
  mutation AdminPublishMaster($id: ID!) {
    adminPublishMasterCardgroup(id: $id) {
      __typename
      ... on PublishMasterCardgroupSuccess {
        master {
          id
          name
          description
          version
          status
          isDefaultStarter
          sortOrder
          cardCount
        }
      }
      ... on MasterCardgroupEmptyError {
        message
      }
    }
  }
`);

export const AdminUnpublishMasterMutation = graphql(`
  mutation AdminUnpublishMaster($id: ID!) {
    adminUnpublishMasterCardgroup(id: $id) {
      id
      name
      description
      version
      status
      isDefaultStarter
      sortOrder
      cardCount
    }
  }
`);

export const AdminDeleteMasterMutation = graphql(`
  mutation AdminDeleteMaster($id: ID!) {
    adminDeleteMasterCardgroup(id: $id)
  }
`);
