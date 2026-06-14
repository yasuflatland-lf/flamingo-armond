"use client";

import dynamic from "next/dynamic";
import type { RefObject } from "react";
import type { CefrLevel } from "@/generated/graphql";
import type { AnimatedCardHandle } from "./animated-card";
import { CefrBadge } from "./cefr-badge";
import type { SwipeDirection } from "./types";
import { useFitText } from "./use-fit-text";

// Front-term auto-fit bounds (px). The front is a single term held on one line
// (`whitespace-nowrap`); useFitText shrinks the font so a long word like
// "cardiovascular" fits the card width instead of wrapping. `max` mirrors the
// previous static sizes (≈ text-5xl unrevealed, ≈ text-3xl revealed); `min`
// is the floor below which the term may overflow the clipped card rather than
// shrink to an unreadable size.
const FRONT_FIT_UNREVEALED = { maxPx: 48, minPx: 20 };
const FRONT_FIT_REVEALED = { maxPx: 30, minPx: 16 };

export type { AnimatedCardHandle } from "./animated-card";

export type SwipeCardData = {
  id: string;
  front: string;
  back: string;
  // CEFR level from the LearnNextDueCards query; `null` = no listed word match.
  cefrLevel: CefrLevel | null;
  userCardState: {
    due: string;
    state: number;
  };
  cardgroupId: string;
};

type Props = {
  card: SwipeCardData;
  isActive: boolean;
  revealed: boolean;
  onReveal: () => void;
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

// CardContent is exported so animated-card.tsx can share the same presentational layer.
export function CardContent({ card, revealed }: { card: SwipeCardData; revealed: boolean }) {
  const { maxPx, minPx } = revealed ? FRONT_FIT_REVEALED : FRONT_FIT_UNREVEALED;
  const { ref: frontRef, fontPx } = useFitText<HTMLParagraphElement>(card.front, maxPx, minPx);

  return (
    // `relative` anchors the absolutely-positioned CefrBadge to this card.
    // The TOP-RIGHT corner is reserved for the CEFR badge; future FSRS badges
    // (#276 / #277) MUST claim a DIFFERENT corner so the two never collide.
    <div className="relative flex h-full flex-col overflow-hidden rounded-lg border border-border bg-card p-6 shadow-lg">
      {/*
        The badge is `position: absolute`, so it never participates in the
        flow and CLS is zero by construction.
      */}
      <CefrBadge level={card.cefrLevel} />
      {/*
        Reserve horizontal space (`px-10`) on the centered content block so a
        long wrapped `front` term cannot slide UNDER the right-pinned badge on
        a narrow (~320px) viewport. Padding does not reflow the absolute badge,
        so CLS stays zero.
      */}
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-8 px-10 text-center">
        <p
          ref={frontRef}
          // Single-line: useFitText measures the term at `maxPx` and shrinks
          // the inline font-size to fit the card width (set via style, so it is
          // a measured pixel value, not a Tailwind step). The outer card is
          // `overflow-hidden`, so a term at the `minPx` floor is clipped rather
          // than spilling past the card edge.
          className="max-w-full whitespace-nowrap font-semibold leading-tight text-foreground"
          style={{ fontSize: `${fontPx}px` }}
        >
          {card.front}
        </p>
        {revealed ? (
          <p className="max-w-full break-words text-xl leading-relaxed text-muted-foreground sm:text-2xl">
            {card.back}
          </p>
        ) : (
          <div className="h-16 w-full max-w-full" aria-hidden="true" />
        )}
      </div>
      {!revealed && (
        <div
          className="pointer-events-none absolute right-0 bottom-0 h-16 w-16 bg-muted shadow-inner [clip-path:polygon(100%_0,0_100%,100%_100%)]"
          aria-hidden="true"
        />
      )}
    </div>
  );
}

export function SwipeCard(props: Props) {
  return <AnimatedCard {...props} />;
}
