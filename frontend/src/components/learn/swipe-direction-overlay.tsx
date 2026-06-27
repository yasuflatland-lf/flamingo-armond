"use client";

import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { RATING_META, type RatingTone, type SwipeDirection } from "./types";

// Overlay-chip color classes keyed by the shared rating tone. These differ from
// the action bar's circular-button classes, so they stay local to this component.
const TONE_CLASS: Record<RatingTone, string> = {
  again: "border-red-500 bg-red-500/10 text-red-700",
  hard: "border-sky-500 bg-sky-500/10 text-sky-700",
  easy: "border-emerald-500 bg-emerald-500/10 text-emerald-700",
};

type Props = {
  direction: SwipeDirection | null;
  intensity: number;
};

export function SwipeDirectionOverlay({ direction, intensity }: Props) {
  const t = useTranslations("Learn");
  if (!direction) return null;
  const { labelKey, tone } = RATING_META[direction];
  const clamped = Math.max(0, Math.min(intensity, 1));

  return (
    <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center">
      <div
        className={cn(
          "rounded-xl border-2 px-10 py-6 text-2xl font-semibold uppercase tracking-wide shadow-sm transition-opacity",
          TONE_CLASS[tone],
        )}
        style={{ opacity: 0.25 + clamped * 0.75 }}
      >
        {t(labelKey)}
      </div>
    </div>
  );
}
