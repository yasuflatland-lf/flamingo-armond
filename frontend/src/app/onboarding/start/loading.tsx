import { OnboardingStartSkeleton } from "./_components/onboarding-start-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/onboarding/start` is being prepared. Preserves the
 * resolved bare-shell layout while its GraphQL data is fetched.
 */
export default function Loading() {
  return <OnboardingStartSkeleton />;
}
