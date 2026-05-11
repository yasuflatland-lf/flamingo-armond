"use client";

import { RotateCcw, Smile, Zap } from "lucide-react";
import type { SwipeDirection } from "@/app/learn/[cardgroupId]/learn-client";
import { cn } from "@/lib/utils";

type Props = {
  onRate: (direction: SwipeDirection) => void;
  disabled?: boolean;
};

const DIRECTIONS = [
  {
    direction: "left",
    label: "Again",
    Icon: RotateCcw,
    shortcut: "ArrowLeft",
    colorClass:
      "border-red-600 text-red-600 hover:bg-red-50 focus-visible:ring-red-500 dark:border-red-400 dark:text-red-400 dark:hover:bg-red-950",
  },
  {
    direction: "down",
    label: "Hard",
    Icon: Zap,
    shortcut: "ArrowDown",
    colorClass:
      "border-sky-600 text-sky-600 hover:bg-sky-50 focus-visible:ring-sky-500 dark:border-sky-400 dark:text-sky-400 dark:hover:bg-sky-950",
  },
  {
    direction: "right",
    label: "Easy",
    Icon: Smile,
    shortcut: "ArrowRight",
    colorClass:
      "border-emerald-600 text-emerald-600 hover:bg-emerald-50 focus-visible:ring-emerald-500 dark:border-emerald-400 dark:text-emerald-400 dark:hover:bg-emerald-950",
  },
] as const;

export function LearnActionBar({ onRate, disabled = false }: Props) {
  return (
    <div className="pointer-events-none sticky bottom-0 z-40 flex justify-center px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] md:pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
      <div className="pointer-events-auto flex items-center gap-4">
        {DIRECTIONS.map(({ direction, label, Icon, shortcut, colorClass }) => (
          <div key={direction} className="flex flex-col items-center gap-1">
            <button
              type="button"
              aria-label={`Rate as ${label}`}
              aria-keyshortcuts={shortcut}
              disabled={disabled}
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
            <span className="text-xs text-muted-foreground">{label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
