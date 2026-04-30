"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useCallback, useMemo, useState } from "react";
import { HandleSwipeMutation } from "@/app/learn/queries";
import { SwipeCardStack } from "@/components/learn/swipe-card-stack";
import { SwipeStatusBar } from "@/components/learn/swipe-status-bar";
import { Button } from "@/components/ui/button";
import type {
  HandleSwipeMutation as HandleSwipeMutationType,
  LearnCardsByCardgroupQuery,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

export type SwipeDirection = "left" | "right" | "down";
type LearnCard = LearnCardsByCardgroupQuery["cardsByCardgroup"][number];
type PerformanceMetrics = HandleSwipeMutationType["handleSwipe"]["metrics"];

const MODE_CONFIG = {
  0: {
    label: "Difficult",
    hint: "Take it slow, smaller batches fit this stretch.",
    className: "border-rose-200 bg-rose-50 text-rose-700",
  },
  1: {
    label: "Default",
    hint: "Next batch is tuned from your current baseline.",
    className: "border-slate-200 bg-slate-50 text-slate-700",
  },
  2: {
    label: "Good",
    hint: "Steady pace, keep the next batch balanced.",
    className: "border-sky-200 bg-sky-50 text-sky-700",
  },
  3: {
    label: "Easy",
    hint: "Strong run, the next batch can move faster.",
    className: "border-emerald-200 bg-emerald-50 text-emerald-700",
  },
  4: {
    label: "In While",
    hint: "You are flying, expect a fuller next batch.",
    className: "border-amber-200 bg-amber-50 text-amber-700",
  },
} as const;

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

function modeConfig(mode: number) {
  return MODE_CONFIG[mode as keyof typeof MODE_CONFIG] ?? MODE_CONFIG[1];
}

function ModeBadge({ mode, metrics }: { mode: number; metrics: PerformanceMetrics }) {
  const config = modeConfig(mode);
  const successRate = Math.round(metrics.successRate * 100);

  return (
    <div className="mx-auto mb-4 flex w-full max-w-xl flex-col gap-2 rounded-lg border border-border bg-background px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-2">
        <span
          className={`rounded-full border px-2.5 py-1 text-xs font-semibold ${config.className}`}
        >
          Mode: {config.label}
        </span>
        <span className="text-muted-foreground">{successRate}% success</span>
      </div>
      <span className="text-muted-foreground">{config.hint}</span>
    </div>
  );
}

type Props = {
  cardgroupId: string;
  initialCards: LearnCard[];
};

export function LearnClient({ cardgroupId, initialCards }: Props) {
  const [queue, setQueue] = useState<LearnCard[]>(initialCards);
  const [completed, setCompleted] = useState(0);
  const [swipeDirection, setSwipeDirection] = useState<SwipeDirection | null>(null);
  const [swipeProgress, setSwipeProgress] = useState(0);
  const [localError, setLocalError] = useState<string | null>(null);
  const [performance, setPerformance] = useState<{
    mode: number;
    metrics: PerformanceMetrics;
  } | null>(null);

  const [handleSwipe, { error, loading }] = useMutation(HandleSwipeMutation);
  const backendError = useMemo(() => getBackendErrorBanner(error), [error]);
  const visibleError = localError ?? backendError;

  const reconcileQueue = useCallback((data: HandleSwipeMutationType | null | undefined) => {
    if (data?.handleSwipe) {
      setQueue(data.handleSwipe.nextCards);
      setPerformance({
        mode: data.handleSwipe.performanceMode,
        metrics: data.handleSwipe.metrics,
      });
    }
  }, []);

  const onSwipe = useCallback(
    async (card: LearnCard, direction: SwipeDirection) => {
      const mode = modeFromDirection(direction);
      setLocalError(null);
      setSwipeDirection(null);
      setSwipeProgress(0);
      setQueue((current) => current.filter((candidate) => candidate.id !== card.id));
      setCompleted((current) => current + 1);

      const remaining = queue.filter((candidate) => candidate.id !== card.id).map(withTypename);

      const result = await handleSwipe({
        variables: { input: { cardId: card.id, cardgroupId, mode } },
        optimisticResponse: {
          __typename: "Mutation",
          handleSwipe: {
            __typename: "SwipeResponse",
            nextCards: remaining,
            performanceMode: performance?.mode ?? 1,
            metrics: performance?.metrics ?? DEFAULT_METRICS,
          },
        },
      }).catch((err) => {
        console.error("[LearnClient] handleSwipe rejected", err);
        setQueue((current) => [card, ...current.filter((candidate) => candidate.id !== card.id)]);
        setCompleted((current) => Math.max(0, current - 1));
        setLocalError("Could not save that swipe. Please try again.");
        return null;
      });

      reconcileQueue(result?.data);
    },
    [cardgroupId, handleSwipe, performance, queue, reconcileQueue],
  );

  if (initialCards.length === 0) {
    return (
      <section className="flex flex-1 items-center justify-center">
        <div className="w-full max-w-md rounded-lg border border-dashed border-border p-8 text-center">
          <h1 className="mb-2 text-xl font-semibold">No cards to learn</h1>
          <p className="mb-6 text-sm text-muted-foreground">
            Add cards to this cardgroup before starting a learning session.
          </p>
          <Button asChild>
            <Link href={`/cardgroups/${cardgroupId}/cards`}>Manage cards</Link>
          </Button>
        </div>
      </section>
    );
  }

  return (
    <section className="flex flex-1 flex-col">
      <SwipeStatusBar completed={completed} remaining={queue.length} saving={loading} />

      {performance ? <ModeBadge mode={performance.mode} metrics={performance.metrics} /> : null}

      {visibleError ? (
        <div
          className="mx-auto mb-4 w-full max-w-xl rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          {visibleError}
        </div>
      ) : null}

      <div className="relative flex min-h-[560px] flex-1 items-center justify-center sm:min-h-[620px]">
        <SwipeCardStack
          cards={queue}
          onCardSwiped={onSwipe}
          onSwipeProgress={(direction, progress) => {
            setSwipeDirection(direction);
            setSwipeProgress(progress);
          }}
          swipeDirection={swipeDirection}
          swipeProgress={swipeProgress}
        />
      </div>
    </section>
  );
}
