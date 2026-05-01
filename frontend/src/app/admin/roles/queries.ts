import { graphql } from "@/generated";

// AdminRoleFieldsFragment and AdminRolesQuery already exist in users/queries.ts.
// Re-export from there so roles/ code can import without going through users/.
// Mutation documents are defined here because they are only used by the roles page.

export { AdminRoleFieldsFragment, AdminRolesQuery } from "@/app/admin/users/queries";

export const AdminCreateRoleMutation = graphql(`
  mutation AdminCreateRole($name: String!) {
    createRole(name: $name) {
      ...AdminRoleFields
    }
  }
`);

export const AdminUpdateRoleMutation = graphql(`
  mutation AdminUpdateRole($id: ID!, $name: String!) {
    updateRole(id: $id, name: $name) {
      ...AdminRoleFields
    }
  }
`);

export const AdminDeleteRoleMutation = graphql(`
  mutation AdminDeleteRole($id: ID!) {
    deleteRole(id: $id)
  }
`);
