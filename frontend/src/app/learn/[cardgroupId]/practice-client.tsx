"use client";

import { useQuery } from "@apollo/client/react";
import { useCallback, useEffect, useRef, useState } from "react";
import { PracticeTodaysCardsQuery as PracticeTodaysCardsDocument } from "@/app/learn/queries";
import { AllCaughtUp } from "@/components/learn/all-caught-up";
import { LearnActionBar } from "@/components/learn/learn-action-bar";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import { SwipeCardStack } from "@/components/learn/swipe-card-stack";
import type { SwipeDirection } from "@/components/learn/types";
import { Button } from "@/components/ui/button";
import type { PracticeTodaysCardsQuery } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { LearnSkeleton } from "./_components/learn-skeleton";
import { advancePracticeQueue, outcomeFromDirection } from "./practice-queue";

type PracticeCard = PracticeTodaysCardsQuery["practiceTodaysCards"][number];

/**
 * Persistent banner shown the whole time practice mode is active. Reminds the
 * learner that practice is FSRS-safe: swipes are not recorded and have no
 * impact on the card's review schedule.
 */
const PRACTICE_BANNER = "Practice — swipes aren't recorded";

/**
 * FSRS-safe practice mode.
 *
 * After the daily learn queue is exhausted, the learner can re-study the cards
 * they already reviewed today. Practice NEVER writes: there is no swipe
 * mutation here, and the module deliberately imports no Apollo mutation hook and
 * no swipe-mutation document. Swiping only re-arranges a purely-local queue via
 * the pure `advancePracticeQueue` policy. A static source-grep test in
 * practice-client.test.tsx enforces this FSRS-safe invariant.
 *
 * Data-loading is single-mode (query-driven). Each round fetches a fresh pool
 * with `fetchPolicy: "network-only"`; "Study again" on completion refetches and
 * re-seeds a new round.
 *
 * Failure handling: any query/refetch error renders a destructive banner with a
 * Retry button (recovers from both initial-load and Study-again refetch
 * failures) and emits one structured `console.error` keyed on the error. While a
 * restart is in flight the skeleton is shown instead of the stale completion
 * screen, so the button never looks like a no-op.
 */
export function PracticeClient({ cardgroupId }: { cardgroupId: string }) {
  const { data, loading, error, refetch } = useQuery(PracticeTodaysCardsDocument, {
    variables: { cardgroupId },
    fetchPolicy: "network-only",
    notifyOnNetworkStatusChange: true,
  });

  // Local round queue. Seeded once per round from the query result. `seededFor`
  // tracks which fetched pool the current queue was seeded from (by object
  // identity of the `practiceTodaysCards` array) so the seed runs exactly once
  // per round and a swipe-shrunk queue is never overwritten by a re-render that
  // carries the same data reference.
  const [queue, setQueue] = useState<PracticeCard[]>([]);
  const seededForRef = useRef<readonly PracticeCard[] | null>(null);

  const pool = data?.practiceTodaysCards;
  useEffect(() => {
    // Only seed from settled data. With `fetchPolicy: "network-only"`, an
    // in-flight refetch keeps `data` pointing at the OLD pool; seeding while
    // `loading` is true would flash the just-finished cards back in. The
    // object-identity guard then ensures we seed each settled pool exactly once
    // and never overwrite a swipe-shrunk queue on a same-reference re-render.
    if (loading) return;
    if (!pool) return;
    if (seededForRef.current === pool) return;
    seededForRef.current = pool;
    setQueue([...pool]);
  }, [pool, loading]);

  // Structured diagnostic log on any query/refetch failure. Keyed on the
  // `[error, cardgroupId]` deps, so it fires once per distinct error object
  // (and once per cardgroupId change, which in practice means a remount onto a
  // different group), not on every render. Per the PII rule we log only the
  // fixed `extensions.code` enum, never `error.message`, which may echo
  // user-authored card content.
  useEffect(() => {
    if (!error) return;
    console.error("[PracticeClient] query failed", {
      cardgroupId,
      codes: liftGraphQLCodes(error),
    });
  }, [error, cardgroupId]);

  const swipeStackRef = useRef<SwipeCardStackHandle | null>(null);

  const onCardSwiped = useCallback((card: PracticeCard, direction: SwipeDirection) => {
    // The ONLY effect of a practice swipe: re-arrange the local queue. No
    // mutation, no network. again/hard re-queue the card a few positions later;
    // easy retires it for the round.
    setQueue((current) => advancePracticeQueue(current, card.id, outcomeFromDirection(direction)));
  }, []);

  const handleRate = useCallback((direction: SwipeDirection) => {
    swipeStackRef.current?.triggerSwipe(direction);
  }, []);

  const studyAgain = useCallback(() => {
    // Restart the round: force a fresh fetch so the seed effect re-runs against
    // a new pool reference. `seededForRef` is reset so even an identical pool
    // array re-seeds the queue.
    seededForRef.current = null;
    setQueue([]);
    // The refetch promise is intentionally discarded. Refetch failures surface
    // via the hook's `error` state, not the promise: verified against
    // @apollo/client v4 that the ObservableQuery emits the error to subscribers
    // (ObservableQuery.js sets `result.error` / `networkStatus: error` /
    // `loading: false` on a network "E" notification), while the promise
    // rejection is pre-handled by Apollo's `preventUnhandledRejection`. So a
    // failed restart renders the error banner + Retry, never a dead end.
    void refetch();
  }, [refetch]);

  const retry = useCallback(() => {
    // Re-run the query after a failure. Works from BOTH failure paths: the
    // initial-load failure (where `data` is undefined and the queue is empty)
    // and a Study-again refetch failure (where stale `data` is retained but the
    // queue was already cleared to `[]`). `seededForRef` is reset so the next
    // settled pool re-seeds even if it is the same array reference as before.
    seededForRef.current = null;
    // Discarded promise — see `studyAgain`: a failed refetch surfaces via the
    // hook's `error` state (verified @apollo/client v4 contract), so the banner
    // re-renders rather than silently dropping the user on a dead screen.
    void refetch();
  }, [refetch]);

  // Error takes priority over the skeleton: a failed initial load OR a failed
  // Study-again refetch both surface the destructive banner with a Retry
  // affordance, never a dead end or a misleading completion screen.
  if (error) {
    return (
      <section className="flex flex-1 items-center justify-center">
        <div
          className="w-full max-w-xl rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          <p>
            {getBackendErrorBanner(error) ??
              "Could not load today's practice cards. Please try again."}
          </p>
          <div className="mt-3">
            <Button type="button" variant="outline" onClick={retry}>
              Retry
            </Button>
          </div>
        </div>
      </section>
    );
  }

  // Skeleton while a request is in flight and the local queue is empty. This
  // covers the initial load (no data yet) AND an in-flight Study-again restart
  // (stale `data` still present but the queue was cleared) — in the restart
  // case the stale `data` must NOT fall through to the completion screen, which
  // would make the button look like a no-op and invite a double-click.
  if (loading && queue.length === 0) {
    return <LearnSkeleton />;
  }

  // Empty pool: the learner has not reviewed any cards today, so there is
  // nothing to practice. Terminal state — no "Study again" (a refetch would
  // return the same empty pool).
  if ((pool?.length ?? 0) === 0) {
    return (
      <AllCaughtUp
        heading="No cards practiced today yet"
        message="Review some cards in a learning session first, then come back to practice them."
      />
    );
  }

  // Round complete: the pool was non-empty but every card has been retired
  // (swiped Easy). Offer another round.
  if (queue.length === 0) {
    return (
      <AllCaughtUp
        heading="Practice complete"
        message="You finished this practice round. Study again for another pass — your progress is never affected."
        onStudyAgain={studyAgain}
      />
    );
  }

  return (
    <section className="grid min-h-0 flex-1 grid-rows-[auto_1fr_auto] gap-3">
      <div
        className="mx-auto w-full max-w-xl rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
        role="status"
      >
        {PRACTICE_BANNER}
      </div>

      <div className="relative flex min-h-0 items-center justify-center overflow-hidden">
        <SwipeCardStack ref={swipeStackRef} cards={queue} onCardSwiped={onCardSwiped} />
      </div>
      <LearnActionBar onRate={handleRate} disabled={queue.length === 0} />
    </section>
  );
}
