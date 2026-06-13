import { graphql } from "@/generated";

/** Default page size for the admin users connection. Must stay in sync between SSR seed and client useQuery/cache reads. */
export const ADMIN_USERS_PAGE_SIZE = 20;

export const AdminUserFieldsFragment = graphql(`
  fragment AdminUserFields on User {
    id
    displayName
    bio
    avatarUrl
  }
`);

export const AdminUserProfileFieldsFragment = graphql(`
  fragment AdminUserProfileFields on User {
    id
    version
    displayName
    bio
    avatarUrl
  }
`);

export const AdminRoleFieldsFragment = graphql(`
  fragment AdminRoleFields on Role {
    id
    name
  }
`);

export const AdminUsersQuery = graphql(`
  query AdminUsers(
    $first: Int
    $after: ID
    $last: Int
    $before: ID
    $search: String
  ) {
    users(first: $first, after: $after, last: $last, before: $before, search: $search) {
      edges {
        cursor
        node {
          ...AdminUserFields
          roles {
            ...AdminRoleFields
          }
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

export const AdminUserQuery = graphql(`
  query AdminUser($id: ID!) {
    adminUser(id: $id) {
      ...AdminUserProfileFields
      roles {
        ...AdminRoleFields
      }
    }
  }
`);

export const AdminRolesQuery = graphql(`
  query AdminRoles {
    roles {
      ...AdminRoleFields
    }
  }
`);

export const AdminEditUserMutation = graphql(`
  mutation AdminEditUser(
    $id: ID!
    $expectedVersion: Int!
    $displayName: String
    $bio: String
    $roleIds: [ID!]!
  ) {
    adminEditUser(
      id: $id
      input: {
        expectedVersion: $expectedVersion
        displayName: $displayName
        bio: $bio
        roleIds: $roleIds
      }
    ) {
      __typename
      ... on AdminEditUserSuccess {
        user {
          ...AdminUserProfileFields
          roles {
            ...AdminRoleFields
          }
        }
      }
      ... on InputValidationError {
        field
        message
      }
      ... on CannotRevokeOwnAdminRoleError {
        message
      }
      ... on ConcurrentUpdateError {
        message
      }
    }
  }
`);

export const AdminDeleteUserMutation = graphql(`
  mutation AdminDeleteUser($id: ID!) {
    adminDeleteUser(id: $id)
  }
`);
