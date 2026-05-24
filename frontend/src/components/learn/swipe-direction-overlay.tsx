"use client";

import { cn } from "@/lib/utils";
import type { SwipeDirection } from "./types";

const hints: Record<SwipeDirection, { label: string; className: string }> = {
  left: { label: "Again", className: "border-red-500 bg-red-500/10 text-red-700" },
  down: { label: "Hard", className: "border-sky-500 bg-sky-500/10 text-sky-700" },
  right: { label: "Easy", className: "border-emerald-500 bg-emerald-500/10 text-emerald-700" },
};

type Props = {
  direction: SwipeDirection | null;
  intensity: number;
};

export function SwipeDirectionOverlay({ direction, intensity }: Props) {
  if (!direction) return null;
  const hint = hints[direction];
  const clamped = Math.max(0, Math.min(intensity, 1));

  return (
    <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center">
      <div
        className={cn(
          "rounded-xl border-2 px-10 py-6 text-2xl font-semibold uppercase tracking-wide shadow-sm transition-opacity",
          hint.className,
        )}
        style={{ opacity: 0.25 + clamped * 0.75 }}
      >
        {hint.label}
      </div>
    </div>
  );
}
