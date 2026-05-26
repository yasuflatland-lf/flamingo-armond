import { AdminUsersSkeleton } from "./admin-users-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for /admin/users is being prepared (the RSC performs an
 * auth check before streaming the client component).
 */
export default function Loading() {
  return <AdminUsersSkeleton />;
}
