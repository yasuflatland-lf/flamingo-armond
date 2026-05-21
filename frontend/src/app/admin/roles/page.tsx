import { graphql } from "@/generated";
import { gqlFetch } from "@/lib/apollo/server";
import { AdminRolesClient, type RoleItem } from "./admin-roles-client";

/**
 * Inline-fields query for the SSR seed (no fragment): useFragment is a React
 * hook and must not run inside a Server Component. AdminRolesClient still uses
 * the fragment via the client-side Apollo cache for mutation responses.
 *
 * admin/layout.tsx enforces the admin gate for every /admin route, so no
 * additional auth check is needed here.
 */
const AdminRolesPageQuery = graphql(`
  query AdminRolesPage {
    roles {
      id
      name
    }
  }
`);

export default async function AdminRolesPage() {
  const data = await gqlFetch(AdminRolesPageQuery, { revalidate: 0 });
  // Cast fragment-masked type to the plain serializable shape; runtime value is already plain.
  const initialRoles = data.roles as unknown as RoleItem[];
  return <AdminRolesClient initialRoles={initialRoles} />;
}
