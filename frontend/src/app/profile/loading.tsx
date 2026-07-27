import { ProfileSkeleton } from "./_components/profile-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/profile` is being prepared. Preserves the resolved page
 * layout while its GraphQL data is fetched.
 */
export default function Loading() {
  return <ProfileSkeleton />;
}
