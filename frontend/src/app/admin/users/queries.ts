import { graphql } from "@/generated";
import type { AdminUsersQueryVariables } from "@/generated/graphql";

/** Default page size for the admin users connection. Must stay in sync between SSR seed and client useQuery/cache reads. */
export const ADMIN_USERS_PAGE_SIZE = 20;

/**
 * Default variables for {@link AdminUsersQuery}. Every read site — RSC seed,
 * client `useQuery`, and cache reads/writes in mutation `update` callbacks —
 * MUST use this object (or spread from it) so Apollo's cache key is identical
 * across all three.
 *
 * See: .claude/rules/pagination.md § "Variables shape MUST match between SSR
 * seed and client cache reads"
 */
export const ADMIN_USERS_DEFAULT_VARS: AdminUsersQueryVariables = {
  first: ADMIN_USERS_PAGE_SIZE,
  search: null,
  roleId: null,
  after: null,
};

/**
 * Slim list-side fragment — only the columns the listing screen renders.
 * `bio` is intentionally excluded because the list does not display it; pulling
 * multi-paragraph user-authored content for every page-of-20 is a meaningful
 * waste. The detail-side fragment below extends this with `bio` for the edit
 * page.
 */
export const AdminUserListFieldsFragment = graphql(`
  fragment AdminUserListFields on User {
    id
    displayName
    avatarUrl
    lastActive
  }
`);

/**
 * Detail-side fragment — extends list fields with `bio` for the edit page.
 * Spreads via `...AdminUserListFields` so the two fragments stay in lock-step
 * for the columns that overlap.
 */
export const AdminUserFieldsFragment = graphql(`
  fragment AdminUserFields on User {
    ...AdminUserListFields
    bio
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
    $roleId: ID
  ) {
    users(first: $first, after: $after, last: $last, before: $before, search: $search, roleId: $roleId) {
      edges {
        cursor
        node {
          ...AdminUserListFields
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
