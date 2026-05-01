import { graphql } from "@/generated";

// AdminRoleFieldsFragment and AdminRolesQuery live in users/queries.ts; re-export
// here so roles/ code does not import through users/. Mutation documents below
// are only used by the roles page.
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
