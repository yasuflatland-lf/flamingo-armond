"use client";

import { gql } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  HandleSwipeMutation,
  LEARN_PAGE_LIMIT,
  LearnNextDueCardsQuery as LearnNextDueCardsDocument,
  SetLastViewedCardgroupMutation,
} from "@/app/learn/queries";
import { AllCaughtUp } from "@/components/learn/all-caught-up";
import { LearnActionBar } from "@/components/learn/learn-action-bar";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import { SwipeCardStack } from "@/components/learn/swipe-card-stack";
import type { SwipeDirection } from "@/components/learn/types";
import type {
  HandleSwipeMutation as HandleSwipeMutationType,
  LearnNextDueCardsQuery,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

type LearnCard = LearnNextDueCardsQuery["learnNextDueCards"][number];
type PerformanceMetrics = HandleSwipeMutationType["handleSwipe"]["metrics"];

/**
 * When `queue.length` falls to this value (or below) and is still non-zero,
 * the background prefetch effect fires another `LearnNextDueCards` request
 * to keep the swipe queue full ahead of the user.
 */
export const PREFETCH_THRESHOLD = 5;

const DEFAULT_METRICS: PerformanceMetrics = {
  __typename: "PerformanceMetrics",
  successRate: 0.5,
  avgDifficulty: 0.5,
  retentionRate: 0.5,
  studyStreak: 0,
  lapseRate: 0,
  reviewCount: 0,
};

function modeFromDirection(direction: SwipeDirection): 1 | 2 | 4 {
  switch (direction) {
    case "left":
      return 1;
    case "down":
      return 2;
    case "right":
      return 4;
  }
}

function withTypename(card: LearnCard): LearnCard & { __typename: "Card" } {
  return { ...card, __typename: "Card" };
}

type Props = {
  cardgroupId: string;
  initialCards: LearnCard[];
  /** The id of the user's `lastViewedCardgroup` at server-render time. */
  lastViewedCardgroupId: string | null;
};

export function LearnClient({ cardgroupId, initialCards, lastViewedCardgroupId }: Props) {
  const [queue, setQueue] = useState<LearnCard[]>(initialCards);
  const queueRef = useRef(queue);
  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);
  const [completed, setCompleted] = useState(0);
  const [localError, setLocalError] = useState<string | null>(null);
  const swipeStackRef = useRef<SwipeCardStackHandle | null>(null);

  const [handleSwipe, { error }] = useMutation(HandleSwipeMutation);
  const backendError = useMemo(() => getBackendErrorBanner(error), [error]);
  const visibleError = localError ?? backendError;

  // Persist this cardgroup as the user's last-viewed cardgroup so the HomePage
  // RSC can land them here on next visit. Frontend skips the network call when
  // the server already reports this cardgroup as last-viewed; the server has
  // no throttle (YAGNI). Errors are non-fatal — learning continues.
  //
  // The mutation is fired via the imperative client API rather than useMutation
  // so the cache update can run regardless of caller render state, and so we
  // can keep the effect's dependency surface narrow.
  //
  // No `optimisticResponse`: setLastViewedCardgroup can return typed errors
  // (InputValidationError, UNAUTHENTICATED) which @apollo/client v3.x does not
  // reliably roll back from optimistic writes — see .claude/rules/pagination.md
  // § "Drop optimisticResponse for mutations that can fail with typed GraphQL errors".
  //
  // `lastDispatchedRef` is a mutable ref (not state) so it can be read and
  // written synchronously — state updates are async and would allow Strict
  // Mode's double-mount to fire two mutations for the same cardgroupId.
  const client = useApolloClient();
  const lastDispatchedRef = useRef<string | null>(null);
  useEffect(() => {
    if (lastViewedCardgroupId === cardgroupId) return;
    if (lastDispatchedRef.current === cardgroupId) return;
    lastDispatchedRef.current = cardgroupId;
    client
      .mutate({
        mutation: SetLastViewedCardgroupMutation,
        variables: { cardgroupId },
        update: (cache, { data }) => {
          // Narrow on __typename before the cache write so an InputValidationError
          // or unknown variant does not silently mutate the cache.
          const payload = data?.setLastViewedCardgroup;
          if (payload?.__typename !== "SetLastViewedCardgroupSuccess") return;
          cache.writeFragment({
            id: cache.identify({
              __typename: "User",
              id: payload.user.id,
            }),
            fragment: gql`
              fragment LastViewedFragment on User {
                lastViewedCardgroup {
                  id
                }
              }
            `,
            data: {
              lastViewedCardgroup: payload.user.lastViewedCardgroup,
            },
          });
        },
      })
      .then((result) => {
        const payload = result.data?.setLastViewedCardgroup;
        if (!payload) {
          console.warn("[learn] setLastViewedCardgroup returned null payload");
          return;
        }
        if (payload.__typename !== "SetLastViewedCardgroupSuccess") {
          console.warn("[learn] setLastViewedCardgroup non-success variant", {
            typename: payload.__typename,
          });
        }
      })
      .catch((err) => {
        // err.message is omitted — backend messages may echo user-authored content.
        // See docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
        console.warn("[learn] setLastViewedCardgroup failed", {
          cardgroupId,
          name: err instanceof Error ? err.name : "unknown",
        });
      });
  }, [cardgroupId, lastViewedCardgroupId, client]);

  // Background prefetch: when the queue drops to PREFETCH_THRESHOLD or below
  // (but is non-empty — an empty queue means the user has finished), fetch
  // the next batch of due cards via a side-channel `client.query` and merge
  // them onto the tail by id. The effect re-fires whenever `queue.length`
  // changes, so a successful handleSwipe response that shrinks the queue
  // triggers another prefetch attempt naturally.
  //
  // - fetchPolicy: "network-only" prevents stale data from the Apollo cache.
  // - prefetchInFlightRef guards against double-firing while the previous
  //   request is still pending. Reset in `finally` so a failed attempt does
  //   not block the next threshold crossing.
  // - handleSwipe.nextCards remains the authoritative replace (FSRS-scheduled
  //   from the server). The natural re-fire on the next queue.length change
  //   re-evaluates whether we still need a prefetch.
  // - Failures are silent (console.warn only) so learning can continue on the
  //   current queue. The warn payload omits err.message — backend messages
  //   may carry user-authored content. See
  //   docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
  // isMountedRef guards against calling `setQueue` on an unmounted component
  // when a prefetch resolves after unmount. The cleanup sets it to false; the
  // setup sets it back to true so React 18 StrictMode double-mount works correctly.
  const isMountedRef = useRef(true);
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  const prefetchInFlightRef = useRef(false);
  useEffect(() => {
    if (queue.length === 0 || queue.length > PREFETCH_THRESHOLD) return;
    if (prefetchInFlightRef.current) return;
    prefetchInFlightRef.current = true;
    client
      .query({
        query: LearnNextDueCardsDocument,
        variables: { cardgroupId, limit: LEARN_PAGE_LIMIT },
        fetchPolicy: "network-only",
      })
      .then((result) => {
        if (!isMountedRef.current) return;
        const incoming = result.data?.learnNextDueCards ?? [];
        if (incoming.length === 0) return;
        setQueue((current) => {
          const seen = new Set(current.map((c) => c.id));
          const merged = [...current];
          for (const card of incoming) {
            if (!seen.has(card.id)) merged.push(card);
          }
          return merged;
        });
      })
      .catch((err) => {
        // err.message is omitted — backend messages may echo user-authored content.
        // See docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
        console.warn("[learn] prefetch failed", {
          cardgroupId,
          name: err instanceof Error ? err.name : "unknown",
        });
      })
      .finally(() => {
        prefetchInFlightRef.current = false;
      });
  }, [queue.length, cardgroupId, client]);

  const onSwipe = useCallback(
    async (card: LearnCard, direction: SwipeDirection) => {
      const mode = modeFromDirection(direction);
      setLocalError(null);
      setQueue((current) => current.filter((candidate) => candidate.id !== card.id));
      setCompleted((current) => current + 1);

      const remaining = queueRef.current
        .filter((candidate) => candidate.id !== card.id)
        .map(withTypename);

      const result = await handleSwipe({
        variables: { input: { cardId: card.id, cardgroupId, mode } },
        // performanceMode and metrics are optimistic placeholders. The SwipeResponse
        // schema requires both fields, so we write zero/no-op values here until the
        // server reconciles the cache. No UI consumer reads them today, but omitting
        // them from the optimistic write would break the codegen-generated type contract.
        optimisticResponse: {
          __typename: "Mutation",
          handleSwipe: {
            __typename: "SwipeResponse",
            nextCards: remaining,
            performanceMode: 1,
            metrics: DEFAULT_METRICS,
          },
        },
      }).catch((err) => {
        // err.message is omitted — backend messages may echo user-authored content.
        // See docs/frontend/rsc-error-handling/substring-matching-sdk-error-strings.md.
        console.error("[LearnClient] handleSwipe rejected", {
          cardId: card.id,
          cardgroupId,
          name: err instanceof Error ? err.name : "unknown",
        });
        setQueue((current) => [card, ...current.filter((candidate) => candidate.id !== card.id)]);
        setCompleted((current) => Math.max(0, current - 1));
        setLocalError("Could not save that swipe. Please try again.");
        return null;
      });

      if (result?.data?.handleSwipe) {
        setQueue(result.data.handleSwipe.nextCards);
      } else if (result !== null) {
        // Mutation resolved (no .catch), but the server payload is missing handleSwipe.
        // The optimistic queue is now the source of truth; surface for operator triage.
        console.warn("[LearnClient] handleSwipe resolved without data", {
          cardId: card.id,
          cardgroupId,
        });
      }
    },
    [cardgroupId, handleSwipe],
  );

  // SwipeCardStack owns the overlay paint + commit-delay timing internally,
  // so handleRate only needs to forward the direction through the imperative
  // handle. Keeping handleRate stable across renders preserves React.memo
  // bailouts on LearnActionBar.
  const handleRate = useCallback((direction: SwipeDirection) => {
    swipeStackRef.current?.triggerSwipe(direction);
  }, []);

  if (queue.length === 0) {
    return <AllCaughtUp />;
  }

  return (
    <section className="grid min-h-0 flex-1 grid-rows-[auto_1fr_auto] gap-3">
      {visibleError ? (
        <div
          className="mx-auto w-full max-w-xl rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          {visibleError}
        </div>
      ) : (
        // Placeholder so the card stays in the 1fr row and the action bar in
        // the trailing auto row when the banner is absent. Without it, grid
        // auto-flow would assign the action bar to the 1fr row.
        <div aria-hidden="true" />
      )}

      <div className="relative flex min-h-0 items-center justify-center overflow-hidden">
        <SwipeCardStack
          ref={swipeStackRef}
          cards={queue}
          onCardSwiped={onSwipe}
          completedCount={completed}
        />
      </div>
      <LearnActionBar onRate={handleRate} disabled={queue.length === 0} />
    </section>
  );
}
