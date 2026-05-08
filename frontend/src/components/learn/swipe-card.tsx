"use client";

import dynamic from "next/dynamic";
import { RotateCcw, Smile, Zap } from "lucide-react";
import type { SwipeDirection } from "@/app/learn/[cardgroupId]/learn-client";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type SwipeCardData = {
  id: string;
  front: string;
  back: string;
  due: string;
  state: number;
  cardgroupId: string;
};

type Props = {
  card: SwipeCardData;
  isActive: boolean;
  onSwipe: (card: SwipeCardData, direction: SwipeDirection) => void;
  onSwipeProgress?: (direction: SwipeDirection | null, progress: number) => void;
};

const directionMeta: Record<
  SwipeDirection,
  { label: string; icon: typeof RotateCcw; className: string }
> = {
  left: { label: "Again", icon: RotateCcw, className: "text-red-700 hover:bg-red-50" },
  down: { label: "Hard", icon: Zap, className: "text-sky-700 hover:bg-sky-50" },
  right: { label: "Easy", icon: Smile, className: "text-emerald-700 hover:bg-emerald-50" },
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
const AnimatedCard = dynamic(
  () => import("./animated-card").then((m) => m.AnimatedCard),
  { ssr: false },
);

// CardContent is exported so animated-card.tsx can share the same presentational layer.
export function CardContent({ card, isActive, onSwipe }: Omit<Props, "onSwipeProgress">) {
  return (
    <div className="flex h-full flex-col rounded-lg border border-border bg-card p-6 shadow-lg">
      <div className="flex flex-1 flex-col items-center justify-center gap-8 text-center">
        <p className="max-w-full break-words text-4xl font-semibold leading-tight text-foreground sm:text-5xl">
          {card.front}
        </p>
        <p className="max-w-full break-words text-xl leading-relaxed text-muted-foreground sm:text-2xl">
          {card.back}
        </p>
      </div>

      <div className="grid grid-cols-3 gap-2 border-t border-border pt-4">
        {(Object.keys(directionMeta) as SwipeDirection[]).map((direction) => {
          const meta = directionMeta[direction];
          const Icon = meta.icon;
          return (
            <Button
              key={direction}
              type="button"
              variant="outline"
              className={cn("h-12", meta.className)}
              disabled={!isActive}
              onClick={() => onSwipe(card, direction)}
              aria-label={meta.label}
            >
              <Icon />
              <span className="hidden sm:inline">{meta.label}</span>
            </Button>
          );
        })}
      </div>
    </div>
  );
}

export function SwipeCard(props: Props) {
  return <AnimatedCard {...props} />;
}
