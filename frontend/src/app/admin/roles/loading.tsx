import { AdminRolesSkeleton } from "./admin-roles-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for /admin/roles is being prepared (the RSC performs a
 * gqlFetch to seed the initial roles list before streaming the client).
 */
export default function Loading() {
  return <AdminRolesSkeleton />;
}
