"use client";

import { RotateCcw, Smile, Zap } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { SwipeDirection } from "./types";

type Props = {
  /**
   * Whether the active card's back is revealed. While `false` (the front_only
   * phase of FLIP_TO_REVEAL mode) the rating buttons render but are disabled —
   * the learner reveals the answer by tapping the card / pressing Space, not via
   * this bar. Defaults to `true` for ALWAYS_VISIBLE and practice contexts.
   */
  revealed?: boolean;
  onRate: (direction: SwipeDirection) => void;
  disabled?: boolean;
};

const DIRECTIONS = [
  {
    direction: "left",
    labelKey: "again",
    Icon: RotateCcw,
    shortcut: "ArrowLeft",
    colorClass:
      "border-red-600 text-red-600 hover:bg-red-50 focus-visible:ring-red-500 dark:border-red-400 dark:text-red-400 dark:hover:bg-red-950",
  },
  {
    direction: "down",
    labelKey: "hard",
    Icon: Zap,
    shortcut: "ArrowDown",
    colorClass:
      "border-sky-600 text-sky-600 hover:bg-sky-50 focus-visible:ring-sky-500 dark:border-sky-400 dark:text-sky-400 dark:hover:bg-sky-950",
  },
  {
    direction: "right",
    labelKey: "easy",
    Icon: Smile,
    shortcut: "ArrowRight",
    colorClass:
      "border-emerald-600 text-emerald-600 hover:bg-emerald-50 focus-visible:ring-emerald-500 dark:border-emerald-400 dark:text-emerald-400 dark:hover:bg-emerald-950",
  },
] as const;

export function LearnActionBar({ revealed = true, onRate, disabled = false }: Props) {
  const t = useTranslations("Learn");
  // While the active card is unrevealed (front_only phase), the rating buttons
  // are inert: the learner must reveal the answer by tapping the card / Space.
  // They stay disabled when the caller's own `disabled` flag is set (e.g. the
  // queue has emptied). Combining both keeps the buttons non-interactive and
  // announced as disabled to assistive tech.
  const ratingDisabled = disabled || !revealed;

  return (
    <div className="pointer-events-none z-40 flex justify-center px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] md:pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
      <div className="pointer-events-auto flex items-center gap-4">
        {DIRECTIONS.map(({ direction, labelKey, Icon, shortcut, colorClass }) => (
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
              colorClass,
            )}
          >
            <Icon className="h-6 w-6" aria-hidden="true" />
          </button>
        ))}
      </div>
    </div>
  );
}
