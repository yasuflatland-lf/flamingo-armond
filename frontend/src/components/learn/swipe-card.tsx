"use client";

import dynamic from "next/dynamic";
import type { RefObject } from "react";
import type { AnimatedCardHandle } from "./animated-card";
import type { SwipeDirection } from "./types";

export type { AnimatedCardHandle } from "./animated-card";

export type SwipeCardData = {
  id: string;
  front: string;
  back: string;
  userCardState: {
    due: string;
    state: number;
  };
  cardgroupId: string;
};

type Props = {
  card: SwipeCardData;
  isActive: boolean;
  onSwipe: (card: SwipeCardData, direction: SwipeDirection) => void;
  onSwipeProgress?: (direction: SwipeDirection | null, progress: number) => void;
  // Travels as a normal prop (not React `ref`) so it survives the next/dynamic
  // boundary — see animated-card.tsx for the rationale. SwipeCard spreads
  // {...props} onto AnimatedCard, so handleRef forwards automatically.
  handleRef?: RefObject<AnimatedCardHandle | null>;
};

// AnimatedCard ships @react-spring/web + @use-gesture/react, which both
// require client-only execution. We load it via next/dynamic (ssr: false)
// to avoid the SSR/hydration mismatch that previously needed useMounted.
//
// Trade-off: a chunk-load failure (network blip after deploy, CDN miss)
// renders the loading fallback (`null`) without surfacing an error UI.
// The card area is briefly blank and the user must navigate away to recover.
// Accepted because: (a) chunk failures are rare in production, (b) the
// learn flow is forgiving — the user can swipe to the next card or
// reload, (c) adding an error fallback complicates the success path's
// rendering for an edge case. Revisit if telemetry shows non-trivial
// chunk-failure rates on /learn.
const AnimatedCard = dynamic(() => import("./animated-card").then((m) => m.AnimatedCard), {
  ssr: false,
});

import { MasteryBadge } from "./mastery-badge";

// CardContent is exported so animated-card.tsx can share the same presentational layer.
export function CardContent({ card }: { card: SwipeCardData }) {
  return (
    <div className="relative flex h-full flex-col overflow-hidden rounded-lg border border-border bg-card p-6 shadow-lg">
      <MasteryBadge state={card.userCardState.state} />
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-8 text-center">
        <p className="max-w-full break-words text-4xl font-semibold leading-tight text-foreground sm:text-5xl">
          {card.front}
        </p>
        <p className="max-w-full break-words text-xl leading-relaxed text-muted-foreground sm:text-2xl">
          {card.back}
        </p>
      </div>
    </div>
  );
}

export function SwipeCard(props: Props) {
  return <AnimatedCard {...props} />;
}
