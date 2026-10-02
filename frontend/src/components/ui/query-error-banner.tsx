import Link from "next/link";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { QueryErrorKind } from "@/lib/apollo/errors";

/**
 * Copy slots for the query-error banner. Callers resolve these from their own
 * i18n namespace (Admin, AdminMasters, Cardgroups, Catalog), so the banner takes
 * resolved strings rather than a namespace-bound `t`.
 */
export type QueryErrorBannerCopy = {
  viewForbidden: string;
  sessionExpired: string;
  signInAgain: string;
  retry: string;
};

type QueryErrorBannerProps = {
  kind: QueryErrorKind | null;
  onRetry: () => void;
  copy: QueryErrorBannerCopy;
  testId: string;
  className?: string;
};

/**
 * Shared three-branch banner for a list query error:
 *  - `forbidden`:       caller lacks the required role — no Retry, since re-issuing
 *                       the same query would fail again.
 *  - `unauthenticated`: session expired mid-page — a degraded banner pointing to
 *                       /login rather than a client-side redirect.
 *  - `banner`:          any other error — the message plus a Retry that re-issues
 *                       the query via `onRetry`.
 *
 * Renders nothing when `kind` is null.
 */
export function QueryErrorBanner({
  kind,
  onRetry,
  copy,
  testId,
  className,
}: QueryErrorBannerProps) {
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
