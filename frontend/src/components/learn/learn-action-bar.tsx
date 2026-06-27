"use client";

import { RotateCcw, Smile, Zap } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { RATING_META, type RatingTone, type SwipeDirection } from "./types";

type Props = {
  onRate: (direction: SwipeDirection) => void;
  disabled?: boolean;
};

const DIRECTIONS: { direction: SwipeDirection; Icon: typeof RotateCcw; shortcut: string }[] = [
  { direction: "left", Icon: RotateCcw, shortcut: "ArrowLeft" },
  { direction: "down", Icon: Zap, shortcut: "ArrowDown" },
  { direction: "right", Icon: Smile, shortcut: "ArrowRight" },
];

// Circular-button color classes keyed by the shared rating tone. These differ
// from the overlay chip's classes, so they stay local to this component.
const TONE_CLASS: Record<RatingTone, string> = {
  again: "border-red-600 text-red-600 hover:bg-red-50 focus-visible:ring-red-500",
  hard: "border-sky-600 text-sky-600 hover:bg-sky-50 focus-visible:ring-sky-500",
  easy: "border-emerald-600 text-emerald-600 hover:bg-emerald-50 focus-visible:ring-emerald-500",
};

export function LearnActionBar({ onRate, disabled = false }: Props) {
  const t = useTranslations("Learn");
  // Rating is available whether or not the card is revealed — the learner may
  // swipe / tap-rate a front-only card, or reveal it first to check the answer.
  // The buttons go inert only when the caller's `disabled` flag is set (e.g. the
  // queue has emptied), announced as disabled to assistive tech.
  const ratingDisabled = disabled;

  return (
    <div className="pointer-events-none z-40 flex justify-center px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] md:pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
      <div className="pointer-events-auto flex items-center gap-4">
        {DIRECTIONS.map(({ direction, Icon, shortcut }) => {
          const { labelKey, tone } = RATING_META[direction];
          return (
            <button
              key={direction}
              type="button"
              aria-label={t("rateAs", { label: t(labelKey) })}
              aria-keyshortcuts={shortcut}
              disabled={ratingDisabled}
              aria-disabled={ratingDisabled}
              onClick={() => onRate(direction)}
              className={cn(
                "inline-flex h-14 w-14 items-center justify-center rounded-full border-2 bg-transparent transition active:scale-95",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2",
                "disabled:cursor-not-allowed disabled:opacity-50",
                TONE_CLASS[tone],
              )}
            >
              <Icon className="h-6 w-6" aria-hidden="true" />
            </button>
          );
        })}
      </div>
    </div>
  );
}
