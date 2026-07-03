"use client";

import dynamic from "next/dynamic";
import { memo, type RefObject } from "react";
import type { CefrLevel } from "@/generated/graphql";
import type { AnimatedCardHandle } from "./animated-card";
import type { SwipeDirection } from "./types";
import { useFitText } from "./use-fit-text";

// Headword auto-fit bounds (px). The headword wraps at word boundaries
// (`break-normal` — never mid-word) and is shrunk by useFitText to fit both the
// card width (so a single long word like "cardiovascular" fits one line) AND a
// maximum line count derived from the word count (see MIN_WORDS_PER_LINE). The
// SAME ceiling is used whether or not the card is revealed, so the headword
// keeps a constant size when the card is flipped — revealing only adds the
// translation below it, it never resizes the term. `minPx` is the floor below
// which an exceptionally long headword is clipped by the card rather than shrunk
// to an unreadable size.
const FRONT_FIT = { maxPx: 48, minPx: 20 };

// Target line density for the headword: shrink the font so the phrase wraps to
// at most ceil(wordCount / MIN_WORDS_PER_LINE) lines. This keeps each line
// fuller (~3 words) and, paired with `text-wrap: balance`, prevents a lone word
// stranded on its own line. A 1–3 word headword targets one line; a 4–6 word
// phrase targets two balanced lines; and so on.
const MIN_WORDS_PER_LINE = 3;

function countWords(text: string): number {
  return text.trim().split(/\s+/).filter(Boolean).length;
}

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
  // Cap the wrapped line count so the headword averages ~MIN_WORDS_PER_LINE
  // words per line; useFitText shrinks the font until it fits within that many
  // lines. `text-wrap: balance` on the element then spreads the words evenly.
  const headwordMaxLines = Math.max(1, Math.ceil(countWords(card.front) / MIN_WORDS_PER_LINE));
  const { ref: frontRef, fontPx } = useFitText<HTMLParagraphElement>(
    card.front,
    FRONT_FIT.maxPx,
    FRONT_FIT.minPx,
    headwordMaxLines,
  );

  return (
    // `relative` anchors the tap-hint dot below. The CEFR badge is NOT rendered
    // here — AnimatedCard overlays it outside the flip rotator (see there). The
    // top-right corner stays reserved for it; future FSRS badges (#276 / #277)
    // must claim a different corner.
    <div className="relative flex h-full flex-col overflow-hidden rounded-lg border border-border bg-card p-6 shadow-lg">
      {/*
        Reserve horizontal space (`px-10`) on the centered content block so a
        long wrapped `front` term cannot slide UNDER the top-right CEFR badge on
        a narrow (~320px) viewport. Padding does not reflow the absolute badge,
        so CLS stays zero.
      */}
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-8 px-10 text-center">
        <p
          ref={frontRef}
          // `break-normal` wraps at word boundaries and never breaks a word
          // mid-character; `text-balance` spreads the words evenly across lines.
          // useFitText measures the term at `maxPx` and shrinks the inline
          // font-size (a measured pixel value, not a Tailwind step) to fit both
          // the card width and the target line count (headwordMaxLines), so a
          // multi-word phrase wraps to fuller, balanced lines instead of
          // stranding a lone word. The outer card is `overflow-hidden`, so a term
          // still too large at the `minPx` floor is clipped rather than spilling.
          className="max-w-full text-balance break-normal font-semibold leading-tight text-foreground"
          style={{ fontSize: `${fontPx}px` }}
        >
          {card.front}
        </p>
        {revealed ? (
          // `break-normal` (not `break-words`): the translation wraps across
          // lines at word boundaries and is never broken mid-word.
          <p className="max-w-full break-normal text-xl leading-relaxed text-muted-foreground sm:text-2xl">
            {card.back}
          </p>
        ) : (
          <div className="h-16 w-full max-w-full" aria-hidden="true" />
        )}
      </div>
      {!revealed && (
        // Tap affordance: a faint coral dot gently pulses (~3s) to hint the card
        // is tappable to reveal the back. Decorative only (aria-hidden); the
        // pulse runs only when motion is allowed and degrades to a static faint
        // dot under prefers-reduced-motion so the hint never disappears.
        <span
          data-testid="tap-hint"
          aria-hidden="true"
          className="pointer-events-none absolute right-5 bottom-5 h-10 w-10 rounded-full bg-brand-primary blur-[10px] motion-safe:animate-tap-pulse motion-reduce:opacity-40"
        />
      )}
    </div>
  );
}

// Memoized so the three stacked SwipeCard instances bail out of re-render while
// a drag is in flight. Every prop the stack passes down — the card object, the
// isActive/revealed booleans, the useCallback([]) handlers, and handleRef — is
// referentially stable across the per-frame drag-progress state updates that
// re-render SwipeCardStack, so a shallow-prop bailout leaves only
// SwipeDirectionOverlay repainting. The card transform itself is driven
// imperatively inside AnimatedCard (api.start with immediate), never through a
// React re-render, so the cards have no reason to re-render mid-gesture.
export const SwipeCard = memo(function SwipeCard(props: Props) {
  return <AnimatedCard {...props} />;
});
