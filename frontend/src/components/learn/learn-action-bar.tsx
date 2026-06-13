"use client";

import { Eye, RotateCcw, Smile, Zap } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { SwipeDirection } from "./types";

type Props = {
  revealed?: boolean;
  onReveal?: () => void;
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

export function LearnActionBar({
  revealed = true,
  onReveal = () => {},
  onRate,
  disabled = false,
}: Props) {
  const t = useTranslations("Learn");
  if (!revealed) {
    return (
      <div className="pointer-events-none z-40 flex justify-center px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] md:pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
        <div className="pointer-events-auto flex w-full max-w-xl items-center">
          <button
            type="button"
            aria-keyshortcuts="Space"
            disabled={disabled}
            onClick={onReveal}
            className={cn(
              "inline-flex h-14 w-full items-center justify-center gap-2 rounded-full border border-primary bg-primary px-6 font-medium text-primary-foreground transition active:scale-[0.99]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2",
              "disabled:cursor-not-allowed disabled:opacity-50",
            )}
          >
            <Eye className="h-5 w-5" aria-hidden="true" />
            <span>{t("showAnswer")}</span>
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="pointer-events-none z-40 flex justify-center px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] md:pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
      <div className="pointer-events-auto flex items-center gap-4">
        {DIRECTIONS.map(({ direction, labelKey, Icon, shortcut, colorClass }) => (
          <button
            key={direction}
            type="button"
            aria-label={t("rateAs", { label: t(labelKey) })}
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
        ))}
      </div>
    </div>
  );
}
