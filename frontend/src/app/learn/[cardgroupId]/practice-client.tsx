"use client";

import { useQuery } from "@apollo/client/react";
import { useCallback, useEffect, useRef, useState } from "react";
import { PracticeTodaysCardsQuery as PracticeTodaysCardsDocument } from "@/app/learn/queries";
import { AllCaughtUp } from "@/components/learn/all-caught-up";
import { LearnActionBar } from "@/components/learn/learn-action-bar";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import { SwipeCardStack } from "@/components/learn/swipe-card-stack";
import type { SwipeDirection } from "@/components/learn/types";
import type { PracticeTodaysCardsQuery } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
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
    if (!pool) return;
    if (seededForRef.current === pool) return;
    seededForRef.current = pool;
    setQueue([...pool]);
  }, [pool]);

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
    void refetch();
  }, [refetch]);

  // Initial load: no data yet and the first request is still in flight.
  if (loading && !data) {
    return <LearnSkeleton />;
  }

  if (error) {
    return (
      <section className="flex flex-1 items-center justify-center">
        <div
          className="w-full max-w-xl rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          {getBackendErrorBanner(error)}
        </div>
      </section>
    );
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
