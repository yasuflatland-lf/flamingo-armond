import { graphql } from "@/generated";

// System role names protected by the backend AdminRole usecase: rename and
// delete are blocked server-side, and the UI mirrors the guard so neither
// affordance is offered. Kept in sync with isSystemRole in
// backend/internal/usecase/admin_user.go.
export const SYSTEM_ROLE_NAMES: ReadonlySet<string> = new Set(["admin", "general"]);

export const AdminRoleQuery = graphql(`
  query AdminRole($id: ID!) {
    role(id: $id) {
      ...AdminRoleFields
    }
  }
`);

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
