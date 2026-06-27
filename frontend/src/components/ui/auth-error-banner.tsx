import Link from "next/link";
import { ErrorBanner } from "@/components/ui/error-banner";

/**
 * Copy slots for the mutation-path auth banner. Callers resolve these from their
 * own i18n namespace: the consuming screens span the Admin / Cardgroups / Catalog
 * / OnboardingStart next-intl namespaces, so the banner takes resolved strings
 * rather than a namespace-bound `t`, keeping it a pure presentational leaf with
 * no i18n coupling. This mirrors AdminQueryErrorBanner (the query-path sibling).
 */
type AuthErrorBannerProps = {
  /** data-testid the consuming screen's tests select on. */
  testId: string;
  /** Resolved unauthenticated-vs-forbidden copy for the consumer's namespace. */
  message: string;
  /** Resolved "Sign in again" link label. */
  signInLabel: string;
  className?: string;
};

/**
 * Shared degraded sign-in banner for a mid-session mutation auth failure
 * (UNAUTHENTICATED / FORBIDDEN). Renders the resolved copy followed by a
 * `<Link href="/login">` rather than a client-side redirect, per
 * `.claude/rules/frontend-rsc-error-handling.md` § "Mid-session UNAUTHENTICATED
 * in a client component: degraded banner with `<Link href="/login">`, not
 * `redirect()`".
 */
export function AuthErrorBanner({ testId, message, signInLabel, className }: AuthErrorBannerProps) {
  return (
    <ErrorBanner className={className} data-testid={testId}>
      <span>{message}</span>{" "}
      <Link href="/login" className="underline">
        {signInLabel}
      </Link>
      .
    </ErrorBanner>
  );
}
