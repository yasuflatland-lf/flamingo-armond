import { graphql } from "@/generated";
import { gqlFetch } from "@/lib/apollo/server";
import { AdminRolesClient, type RoleItem } from "./AdminRolesClient";

/**
 * Fragment-less query for the SSR seed. Using inline fields instead of
 * the AdminRoleFields fragment avoids calling useFragment (a React hook)
 * inside a Server Component. The fragment is still used by mutation
 * responses in AdminRolesClient via the client-side Apollo cache.
 *
 * admin/layout.tsx already enforces the admin gate for every route under
 * /admin, so no additional auth check is needed here.
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
  // Cast from fragment-masked type to the plain serializable shape the
  // client component expects. The runtime value is already the plain object.
  const initialRoles = data.roles as unknown as RoleItem[];
  return <AdminRolesClient initialRoles={initialRoles} />;
}
