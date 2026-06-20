import type { RefObject } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";

interface ConnectionListFooterProps {
  /** IntersectionObserver sentinel ref owned by `useConnectionPagination`. */
  sentinelRef: RefObject<HTMLDivElement | null>;
  /** Banner string when a `fetchMore` failed; `null` while healthy. */
  fetchMoreError: string | null;
  /** Re-invokes the failed `fetchMore` (the hook's `retryFetchMore`). */
  onRetry: () => void;
  /** True while a `fetchMore` is in flight. */
  fetchingMore: boolean;
  /** Whether more pages remain (gates the "loading more" line). */
  hasNextPage: boolean;
  /** Localized Retry button label (e.g. `tCommon("retry")`). */
  retryLabel: string;
  /** Localized "loading more" line (e.g. `t("loadingMore")`). */
  loadingMoreLabel: string;
  /**
   * Per-screen testid namespace. Reproduces each list's existing ids:
   * `{prefix}-sentinel`, `{prefix}-fetch-more-error`, `{prefix}-loading-more`.
   */
  testIdPrefix: string;
}

/**
 * Shared pagination footer for every list screen that consumes
 * `useConnectionPagination`. Renders the three duplicated pieces — the
 * IntersectionObserver sentinel, a `fetchMore`-error banner with a canonical
 * `<Button variant="outline">` Retry, and the "loading more" line — so the
 * chrome around the IO loop lives in one place.
 *
 * Purely presentational: localized labels are passed in (the component calls no
 * `useTranslations`), and the sentinel ref is owned by the hook (the observer
 * attaches to the DOM node via the ref, so the tree position here is irrelevant).
 */
export function ConnectionListFooter({
  sentinelRef,
  fetchMoreError,
  onRetry,
  fetchingMore,
  hasNextPage,
  retryLabel,
  loadingMoreLabel,
  testIdPrefix,
}: ConnectionListFooterProps) {
  return (
    <>
      <div ref={sentinelRef} aria-hidden="true" data-testid={`${testIdPrefix}-sentinel`} />

      {fetchMoreError && (
        <ErrorBanner
          className="mt-3 flex flex-col items-center gap-2"
          data-testid={`${testIdPrefix}-fetch-more-error`}
        >
          <span>{fetchMoreError}</span>
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            {retryLabel}
          </Button>
        </ErrorBanner>
      )}

      {!fetchMoreError && fetchingMore && hasNextPage && (
        <p
          className="mt-3 text-center text-xs text-muted-foreground"
          data-testid={`${testIdPrefix}-loading-more`}
        >
          {loadingMoreLabel}
        </p>
      )}
    </>
  );
}
