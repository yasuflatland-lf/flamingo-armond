"use client";

import { gql } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  HandleSwipeMutation,
  LEARN_PAGE_LIMIT,
  LearnNextDueCardsQuery as LearnNextDueCardsDocument,
  SetLastViewedCardgroupMutation,
} from "@/app/learn/queries";
import { AllCaughtUp } from "@/components/learn/all-caught-up";
import { SwipeSession } from "@/components/learn/swipe-session";
import type { LearnDisplayMode, SwipeDirection } from "@/components/learn/types";
import { RATING_META, SWIPE_RATING } from "@/components/learn/types";
import { ErrorBanner } from "@/components/ui/error-banner";
import type { LearnNextDueCardsQuery } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { learnDayKey } from "@/lib/learn/learn-day";
import { mergePrefetchedCards } from "./learn-queue";
import { PracticeClient } from "./practice-client";

type LearnCard = LearnNextDueCardsQuery["learnNextDueCards"][number];
/**
 * When `queue.length` falls to this value (or below) and is still non-zero,
 * the background prefetch effect fires another `LearnNextDueCards` request
 * to keep the swipe queue full ahead of the user.
 */
export const PREFETCH_THRESHOLD = 5;

function ratingFromDirection(direction: SwipeDirection): 1 | 2 | 4 {
  return SWIPE_RATING[RATING_META[direction].tone];
}

type Props = {
  cardgroupId: string;
  initialCards: LearnCard[];
  displayMode: LearnDisplayMode;
};

export function LearnClient({ cardgroupId, initialCards, displayMode }: Props) {
  const t = useTranslations("Learn");
  const [queue, setQueue] = useState<LearnCard[]>(initialCards);
  const [completedCount, setCompletedCount] = useState(0);
  const [localError, setLocalError] = useState<string | null>(null);
  // Top-level phase switch. "practice" hands the whole screen to PracticeClient
  // (FSRS-safe re-study of today's cards). It is only entered from the
  // AllCaughtUp "Study again" action when the daily learn queue is exhausted.
  const [phase, setPhase] = useState<"learn" | "practice">("learn");

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
  // Ids of every card swiped in this session, scoped to the cardgroup and to the
  // JST learn day tracked by `learnDayRef`. `mergePrefetchedCards` filters each
  // prefetched batch against this set; the reasoning for why an already-swiped
  // card can come back from the server at all, and why the ids stay in the set
  // for a whole learn day rather than being pruned one by one, lives with the
  // policy in `./learn-queue`.
  //
  // A transport failure keeps its id here too, which is harmless: the catch
  // handler puts the card back at the queue head, so the queue itself covers it
  // for as long as it is queued and the learner can re-swipe it.
  const swipedThisSessionRef = useRef<Set<string>>(new Set());
  // The exhaustion verdict returned by the last resolved merge: the due pool
  // added nothing, so every subsequent tail swipe would otherwise fire a
  // redundant network-only query that returns nothing new. The effect
  // short-circuits while this is set.
  //
  // Cleared after a swipe mutation succeeds, on cardgroup change, and at the JST
  // learn-day rollover (which refills the due pool wholesale). The reset is
  // not about the just-swiped card — that one can never come back today — but
  // about the verdict's age: it is a point-in-time snapshot, and the filler
  // review window admits a card once `due <= now`, so the pool can refill with
  // newly-due cards while the session runs.
  const exhaustedRef = useRef(false);
  // Tracks the cardgroup the exhaustion verdict belongs to. When the active
  // cardgroup changes, the prefetch effect below clears `exhaustedRef` before
  // its threshold checks, because a different deck has its own due pool and a
  // previous "nothing due" verdict must not carry over.
  const exhaustedForCardgroupRef = useRef(cardgroupId);
  // Tracks the JST learn day both session guards belong to. The server rolls the
  // learn day over at JST midnight and re-serves the previous day's cards, so a
  // session that outlives the boundary must drop the swiped-id set (or every
  // re-served card is filtered out) and the exhaustion verdict (or the refilled
  // queue is never fetched). The key is re-read at the start of every swipe and
  // on each run of the prefetch effect below.
  const learnDayRef = useRef(learnDayKey());

  // Drops both session guards when the JST learn day has advanced since they
  // were last keyed. This runs at the top of `onSwipe` as well as inside the
  // prefetch effect: an effect-only check would fire for the first time on the
  // commit caused by the boundary-crossing swipe, which has already recorded its
  // own id, and would therefore erase exactly the guard that stops the racing
  // prefetch from re-appending that card. Resetting before the id is recorded
  // re-registers it into the fresh set, while ids swiped on the previous learn
  // day are still dropped.
  const syncLearnDay = useCallback(() => {
    const today = learnDayKey();
    if (learnDayRef.current === today) return;
    learnDayRef.current = today;
    exhaustedRef.current = false;
    swipedThisSessionRef.current = new Set();
  }, []);

  useEffect(() => {
    if (exhaustedForCardgroupRef.current !== cardgroupId) {
      exhaustedForCardgroupRef.current = cardgroupId;
      exhaustedRef.current = false;
      swipedThisSessionRef.current = new Set();
    }
    syncLearnDay();
    if (queue.length === 0 || queue.length > PREFETCH_THRESHOLD) return;
    if (prefetchInFlightRef.current) return;
    if (exhaustedRef.current) return;
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
        // The merge runs inside the updater so it always sees the freshest
        // queue: `onSwipe`'s optimistic delete can land between this request
        // being issued and its response arriving. Writing `exhaustedRef` here
        // is idempotent, so Strict Mode's double-invoke of the updater is
        // harmless. All queue policy lives in `mergePrefetchedCards`.
        setQueue((current) => {
          const merged = mergePrefetchedCards(
            current,
            incoming,
            { dayKey: learnDayRef.current, ids: swipedThisSessionRef.current },
            learnDayRef.current,
          );
          exhaustedRef.current = merged.exhausted;
          return merged.queue;
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
  }, [queue.length, cardgroupId, client, syncLearnDay]);

  const onSwipe = useCallback(
    async (card: LearnCard, direction: SwipeDirection) => {
      const rating = ratingFromDirection(direction);
      setLocalError(null);
      // Record the card as swiped BEFORE the optimistic removal so the prefetch
      // effect (which the removal can re-fire) filters it out of any racing
      // LearnNextDueCards batch. Recording at swipe time rather than on success
      // also closes the gap between the mutation settling and its continuation
      // running. The id stays in the set until the JST learn day rolls over —
      // see the ref's declaration above. The rollover check runs FIRST so a
      // swipe that crosses JST midnight records its id into the freshly-cleared
      // set, instead of having it erased by the effect this very swipe re-fires.
      syncLearnDay();
      swipedThisSessionRef.current.add(card.id);
      setQueue((current) => current.filter((candidate) => candidate.id !== card.id));
      setCompletedCount((current) => current + 1);

      const result = await handleSwipe({
        variables: { input: { cardId: card.id, cardgroupId, rating } },
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
        setCompletedCount((current) => Math.max(0, current - 1));
        setLocalError(t("swipeSaveFailed"));
        return null;
      });

      if (!result) return;

      // HandleSwipeSuccess is a no-op: the optimistic delete already advanced
      // the queue and the response carries only performance telemetry that no
      // UI consumer reads today. Only the non-success branches need handling.
      const payload = result.data?.handleSwipe;
      if (payload?.__typename === "HandleSwipeSuccess") {
        // Re-open prefetching in case the pool was previously marked exhausted.
        // That verdict may predate cards that have since become due (the filler
        // review window admits a card once its `due` has arrived); the
        // just-swiped card itself never returns today.
        exhaustedRef.current = false;
        return;
      }

      if (payload?.__typename === "InputValidationError") {
        // Server rejected the swipe (stale card, cardgroup mismatch, invalid rating).
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
    [cardgroupId, handleSwipe, syncLearnDay, t],
  );

  if (phase === "practice") {
    return <PracticeClient cardgroupId={cardgroupId} />;
  }

  if (queue.length === 0) {
    return <AllCaughtUp onStudyAgain={() => setPhase("practice")} />;
  }

  return (
    <SwipeSession
      cards={queue}
      displayMode={displayMode}
      onCardSwiped={onSwipe}
      completedCount={completedCount}
      // When no banner, SwipeSession renders an `aria-hidden` spacer in the
      // leading row so the card and action bar keep their grid rows.
      topSlot={
        visibleError ? (
          <ErrorBanner className="mx-auto w-full max-w-xl">{visibleError}</ErrorBanner>
        ) : undefined
      }
    />
  );
}
