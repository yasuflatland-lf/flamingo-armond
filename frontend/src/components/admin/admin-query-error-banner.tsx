import Link from "next/link";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { QueryErrorKind } from "@/lib/apollo/errors";

/**
 * Copy slots for the admin query-error banner. Callers resolve these from their
 * own i18n namespace: the admin list screens use different next-intl namespaces
 * (`AdminMasters`, `Admin`) that share the same key names, so the banner takes
 * resolved strings rather than a namespace-bound `t`, keeping it a pure
 * presentational leaf with no i18n coupling.
 */
type AdminQueryErrorBannerCopy = {
  viewForbidden: string;
  sessionExpired: string;
  signInAgain: string;
  retry: string;
};

type AdminQueryErrorBannerProps = {
  kind: QueryErrorKind | null;
  onRetry: () => void;
  copy: AdminQueryErrorBannerCopy;
  testId: string;
  className?: string;
};

/**
 * Shared three-branch banner for an admin list query error:
 *  - `forbidden`:       caller lacks the admin role — no Retry, since re-issuing
 *                       the same query would fail again.
 *  - `unauthenticated`: session expired mid-page — a degraded banner pointing to
 *                       /login rather than a client-side redirect.
 *  - `banner`:          any other error — the message plus a Retry that re-issues
 *                       the query via `onRetry`.
 *
 * Renders nothing when `kind` is null.
 */
export function AdminQueryErrorBanner({
  kind,
  onRetry,
  copy,
  testId,
  className,
}: AdminQueryErrorBannerProps) {
  if (!kind) return null;

  if (kind.kind === "forbidden") {
    return (
      <ErrorBanner className={className} data-testid={testId}>
        {copy.viewForbidden}
      </ErrorBanner>
    );
  }

  if (kind.kind === "unauthenticated") {
    return (
      <ErrorBanner className={className} data-testid={testId}>
        <span>{copy.sessionExpired}</span>{" "}
        <Link href="/login" className="underline">
          {copy.signInAgain}
        </Link>
      </ErrorBanner>
    );
  }

  return (
    <ErrorBanner className={className} data-testid={testId}>
      <span>{kind.message}</span>
      <button type="button" className="ml-3 underline" onClick={() => onRetry()}>
        {copy.retry}
      </button>
    </ErrorBanner>
  );
}
