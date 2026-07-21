import type { Metadata } from "next";
import { graphql } from "@/generated";
import type { AdminRolesPageQuery as AdminRolesPageQueryType } from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { AdminRolesClient, type RoleItem } from "./admin-roles-client";

// Admin-only route — must not be indexed.
export const metadata: Metadata = {
  title: "Roles",
  robots: { index: false, follow: false },
};

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
  let data: AdminRolesPageQueryType;
  try {
    data = await gqlFetch(AdminRolesPageQuery, { revalidate: 0 });
  } catch (err) {
    // Admin pages redirect to "/" (not "/login"), matching admin/layout.tsx
    // and admin/users/page.tsx. UNAUTHENTICATED / FORBIDDEN fold into the same
    // redirect path; everything else is logged (PII-redacted) and rethrown.
    redirectIfAuthError(err, "/", { forbidden: true });
    console.error("[admin/roles] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }
  // Cast fragment-masked type to the plain serializable shape; runtime value is already plain.
  const initialRoles = data.roles as unknown as RoleItem[];
  return <AdminRolesClient initialRoles={initialRoles} />;
}
