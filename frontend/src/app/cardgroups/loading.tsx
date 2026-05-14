import { CardgroupsSkeleton } from "./_components/cardgroups-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/cardgroups` is being prepared. Uses the same skeleton
 * the in-page `<Suspense>` boundary renders, so the layout shape is identical
 * across both the navigation transition and the streaming fallback.
 */
export default function Loading() {
  return <CardgroupsSkeleton />;
}
