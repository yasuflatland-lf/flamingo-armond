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
      ...AdminUserFields
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

export const AdminUpdateUserMutation = graphql(`
  mutation AdminUpdateUser($id: ID!, $input: AdminUpdateUserInput!) {
    adminUpdateUser(id: $id, input: $input) {
      ...AdminUserFields
      roles {
        ...AdminRoleFields
      }
    }
  }
`);

export const AdminAssignRoleMutation = graphql(`
  mutation AdminAssignRole($userId: ID!, $roleId: ID!) {
    assignRole(userId: $userId, roleId: $roleId) {
      ...AdminUserFields
      roles {
        ...AdminRoleFields
      }
    }
  }
`);

export const AdminRevokeRoleMutation = graphql(`
  mutation AdminRevokeRole($userId: ID!, $roleId: ID!) {
    revokeRole(userId: $userId, roleId: $roleId) {
      ...AdminUserFields
      roles {
        ...AdminRoleFields
      }
    }
  }
`);
