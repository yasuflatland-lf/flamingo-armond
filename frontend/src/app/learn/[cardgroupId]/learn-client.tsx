"use client";

import { gql } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { HandleSwipeMutation, SetLastViewedCardgroupMutation } from "@/app/learn/queries";
import { SwipeCardStack } from "@/components/learn/swipe-card-stack";
import { LearnAddCardFloating } from "@/components/nav/learn-add-card-floating";
import { Button } from "@/components/ui/button";
import type {
  HandleSwipeMutation as HandleSwipeMutationType,
  LearnCardsByCardgroupQuery,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

export type SwipeDirection = "left" | "right" | "down";
type LearnCard = LearnCardsByCardgroupQuery["cardsByCardgroup"][number];
type PerformanceMetrics = HandleSwipeMutationType["handleSwipe"]["metrics"];

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
  cardgroupName: string;
  initialCards: LearnCard[];
  /** The id of the user's `lastViewedCardgroup` at server-render time. */
  lastViewedCardgroupId: string | null;
};

export function LearnClient({ cardgroupId, cardgroupName, initialCards, lastViewedCardgroupId }: Props) {
  const [queue, setQueue] = useState<LearnCard[]>(initialCards);
  const [completed, setCompleted] = useState(0);
  const [swipeDirection, setSwipeDirection] = useState<SwipeDirection | null>(null);
  const [swipeProgress, setSwipeProgress] = useState(0);
  const [localError, setLocalError] = useState<string | null>(null);

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
  // (BAD_USER_INPUT, UNAUTHENTICATED) which @apollo/client v3.x does not
  // reliably roll back from optimistic writes — see pagination.md.
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
          if (!data?.setLastViewedCardgroup) return;
          cache.writeFragment({
            id: cache.identify({
              __typename: "User",
              id: data.setLastViewedCardgroup.id,
            }),
            fragment: gql`
              fragment LastViewedFragment on User {
                lastViewedCardgroup {
                  id
                }
              }
            `,
            data: {
              lastViewedCardgroup: data.setLastViewedCardgroup.lastViewedCardgroup,
            },
          });
        },
      })
      .catch((err) => {
        console.warn("[learn] setLastViewedCardgroup failed", { cardgroupId, err });
      });
  }, [cardgroupId, lastViewedCardgroupId, client]);

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
        console.error("[LearnClient] handleSwipe rejected", err);
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
    [cardgroupId, handleSwipe, queue],
  );

  if (initialCards.length === 0) {
    return (
      <>
        <LearnAddCardFloating cardgroupId={cardgroupId} cardgroupName={cardgroupName} />
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
      </>
    );
  }

  return (
    <>
      <LearnAddCardFloating cardgroupId={cardgroupId} cardgroupName={cardgroupName} />
      <section className="flex flex-1 flex-col">
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
          completedCount={completed}
        />
      </div>
    </section>
    </>
  );
}
