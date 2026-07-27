import { ChangeEmailSkeleton } from "./_components/change-email-skeleton";

/**
 * Route-segment navigation fallback for `/profile/change-email`. Preserves the
 * resolved form layout while authentication and translations are prepared.
 */
export default function Loading() {
  return <ChangeEmailSkeleton />;
}
