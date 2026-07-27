import { StatsSkeleton } from "./_components/stats-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/stats` is being prepared. Preserves the resolved page
 * layout while its GraphQL data is fetched.
 */
export default function Loading() {
  return <StatsSkeleton />;
}
