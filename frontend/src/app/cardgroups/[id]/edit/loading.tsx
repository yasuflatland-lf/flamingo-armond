import { CardgroupEditSkeleton } from "./_components/cardgroup-edit-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/cardgroups/[id]/edit` is being prepared. Preserves the
 * resolved editor layout while its GraphQL data is fetched.
 */
export default function Loading() {
  return <CardgroupEditSkeleton />;
}
