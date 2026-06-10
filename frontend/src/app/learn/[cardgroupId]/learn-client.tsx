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
import type { LearnNextDueCardsQuery } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { PracticeClient } from "./practice-client";

type LearnCard = LearnNextDueCardsQuery["learnNextDueCards"][number];
/**
 * When `queue.length` falls to this value (or below) and is still non-zero,
 * the background prefetch effect fires another `LearnNextDueCards` request
 * to keep the swipe queue full ahead of the user.
 */
export const PREFETCH_THRESHOLD = 5;

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

type Props = {
  cardgroupId: string;
  initialCards: LearnCard[];
};

export function LearnClient({ cardgroupId, initialCards }: Props) {
  const [queue, setQueue] = useState<LearnCard[]>(initialCards);
  const [completed, setCompleted] = useState(0);
  const [localError, setLocalError] = useState<string | null>(null);
  // Top-level phase switch. "practice" hands the whole screen to PracticeClient
  // (FSRS-safe re-study of today's cards). It is only entered from the
  // AllCaughtUp "Study again" action when the daily learn queue is exhausted.
  const [phase, setPhase] = useState<"learn" | "practice">("learn");
  const swipeStackRef = useRef<SwipeCardStackHandle | null>(null);

  const [handleSwipe, { error }] = useMutation(HandleSwipeMutation);
  const backendError = useMemo(() => getBackendErrorBanner(error), [error]);
  const visibleError = localError ?? backendError;

  // Persist this cardgroup as the user's last-viewed cardgroup so the HomePage
  // RSC can land them here on next visit. The sync runs once per mount and is
  // fire-and-forget: the page deliberately does NOT fetch the current
  // last-viewed value (that would put an extra query on the LCP critical path),
  // so the mutation always fires even when the value is already current. The
  // server has no throttle (YAGNI) and the redundant write is harmless. Errors
  // are non-fatal — learning continues.
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
          codes: liftGraphQLCodes(err),
        });
      });
  }, [cardgroupId, client]);

  // Background prefetch: the ONLY mechanism that refills the queue.
  //
  // Contract:
  // - The optimistic `setQueue` in `onSwipe` is the sole queue-advance step;
  //   the `handleSwipe` mutation response does NOT modify the queue.
  // - When `queue.length` drops to PREFETCH_THRESHOLD or below (but is
  //   non-zero — empty queue means finished), a side-channel `client.query`
  //   fetches the next batch of FSRS-scheduled cards and merges them onto the
  //   tail by id, deduplicating against what is already in the queue.
  // - The effect re-fires on every `queue.length` change, so each optimistic
  //   delete naturally re-evaluates whether another prefetch is needed.
  //
  // - fetchPolicy: "network-only" prevents stale data from the Apollo cache.
  // - prefetchInFlightRef guards against double-firing while the previous
  //   request is still pending. Reset in `finally` so a failed attempt does
  //   not block the next threshold crossing.
  // - Failures are silent (console.warn only) so learning can continue on the
  //   current queue. The warn payload omits err.message — backend messages
  //   may carry user-authored content. See
  //   docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
  // - isMountedRef guards against calling `setQueue` on an unmounted component
  //   when a prefetch resolves after unmount. The cleanup sets it to false; the
  //   setup sets it back to true so React 18 StrictMode double-mount works correctly.
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

      const result = await handleSwipe({
        variables: { input: { cardId: card.id, cardgroupId, mode } },
        // optimisticResponse intentionally omitted — handleSwipe can return InputValidationError
        // and Apollo v3 does not reliably roll back optimistic writes on typed GraphQL errors.
        // See .claude/rules/pagination.md § "Drop `optimisticResponse` for mutations that can
        // fail with typed GraphQL errors". The optimistic queue advance above (via setQueue)
        // is React state and is unaffected.
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

      if (!result) return;

      // HandleSwipeSuccess is a no-op: the optimistic delete already advanced
      // the queue and the response carries only performance telemetry that no
      // UI consumer reads today. Only the non-success branches need handling.
      const payload = result.data?.handleSwipe;
      if (payload?.__typename === "HandleSwipeSuccess") return;

      if (payload?.__typename === "InputValidationError") {
        // Server rejected the swipe (stale card, cardgroup mismatch, invalid mode).
        // The optimistic queue advanced so learning continues, but we surface to
        // operator telemetry — repeated firing indicates a stale prefetch.
        // payload.message is omitted — it may echo user-authored card content.
        // See docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
        console.warn("[LearnClient] handleSwipe InputValidationError", {
          cardId: card.id,
          cardgroupId,
          field: payload.field,
        });
        return;
      }

      // Unknown variant or null/undefined payload — optimistic queue is now source of truth.
      // Cast through unknown because TypeScript narrows this branch to `never` once all
      // discriminated union members are handled above.
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[LearnClient] handleSwipe unexpected payload", {
        typename: unknownPayload?.__typename ?? null,
        cardId: card.id,
        cardgroupId,
      });
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

  if (phase === "practice") {
    return <PracticeClient cardgroupId={cardgroupId} />;
  }

  if (queue.length === 0) {
    return <AllCaughtUp onStudyAgain={() => setPhase("practice")} />;
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
