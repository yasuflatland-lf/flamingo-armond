"use client";

import { animated, useSpring } from "@react-spring/web";
import { useDrag } from "@use-gesture/react";
import { Trash2 } from "lucide-react";
import { forwardRef, useCallback, useImperativeHandle, useRef } from "react";
import { useReducedMotion } from "@/lib/use-reduced-motion";

// ---------------------------------------------------------------------------
// Public API types
// ---------------------------------------------------------------------------

export interface SwipeableRowHandle {
  /** Programmatically snap the row back to resting position. */
  close(): void;
}

export interface SwipeableRowProps {
  /** Row content — the card front/back text, checkboxes, etc. */
  children: React.ReactNode;
  /** Fires when the user commits a delete gesture. */
  onDelete: () => void;
  /**
   * When true, swipe gesture is completely disabled so the selection-mode
   * checkboxes receive touch events without interference.
   */
  disabled?: boolean;
}

// ---------------------------------------------------------------------------
// Thresholds
// ---------------------------------------------------------------------------

/** Fraction of row width at which a released left-swipe commits delete. */
const COMMIT_THRESHOLD = 0.4;

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * Unidirectional (left-only) swipe-to-delete row.
 *
 * Behaviour:
 * - Swipe left >= 40% of row width then release → row flies off screen →
 *   `onDelete()` fires.
 * - Swipe left < 40% then release → snaps back; `onDelete()` not called.
 * - Predominantly vertical drag → no-op; browser scroll passes through.
 * - `disabled` → gesture is a no-op; touch events pass through to children.
 * - `prefers-reduced-motion: reduce` → swipe layer not rendered; children are
 *   returned as-is. The parent is responsible for the delete affordance in that
 *   case (e.g. a hover Delete icon that is always-visible on reduced-motion).
 *
 * The component exposes an imperative `close()` handle via `ref` so a parent
 * list can snap back any in-progress row when needed.
 */
export const SwipeableRow = forwardRef<SwipeableRowHandle, SwipeableRowProps>(function SwipeableRow(
  { children, onDelete, disabled = false },
  ref,
) {
  const reducedMotion = useReducedMotion();

  if (reducedMotion) {
    return <>{children}</>;
  }

  return (
    <SwipeableRowInner ref={ref} onDelete={onDelete} disabled={disabled}>
      {children}
    </SwipeableRowInner>
  );
});

// ---------------------------------------------------------------------------
// Inner animated implementation (only mounted when motion is allowed)
// ---------------------------------------------------------------------------

const SwipeableRowInner = forwardRef<SwipeableRowHandle, SwipeableRowProps>(
  function SwipeableRowInner({ children, onDelete, disabled }, ref) {
    const rowRef = useRef<HTMLDivElement>(null);

    const [{ x }, api] = useSpring(() => ({
      x: 0,
      config: { tension: 520, friction: 38 },
    }));

    const startDeleteAnimation = useCallback(
      (onComplete: () => void) => {
        const rowWidth = rowRef.current?.offsetWidth ?? 300;
        Promise.all(api.start({ x: -rowWidth }))
          .then(() => {
            onComplete();
          })
          .catch((err) => {
            console.warn("[swipeable-row] delete animation interrupted; committing anyway", {
              name: err instanceof Error ? err.name : "unknown",
            });
            onComplete();
          });
      },
      [api],
    );

    useImperativeHandle(ref, () => ({
      close() {
        api.start({ x: 0 });
      },
    }));

    const bind = useDrag(
      ({ active, movement: [mx, my], last, cancel }) => {
        if (disabled) {
          cancel();
          return;
        }

        // Ignore predominantly vertical movements so the user can still scroll.
        // Treat as horizontal only when |dx| >= |dy|.
        const isHorizontal = Math.abs(mx) >= Math.abs(my);
        if (!isHorizontal && !last) return;

        const clampedMx = Math.min(mx, 0); // clamp to leftward movement only

        const rowWidth = rowRef.current?.offsetWidth ?? 300;
        const fraction = Math.abs(clampedMx) / rowWidth;

        if (last) {
          if (isHorizontal && fraction >= COMMIT_THRESHOLD) {
            startDeleteAnimation(onDelete);
          } else {
            api.start({ x: 0 });
          }
          return;
        }

        if (active) {
          api.start({ x: clampedMx, immediate: true });
        }
      },
      {
        filterTaps: true,
        pointer: { capture: false },
        rubberband: true,
        threshold: 4,
      },
    );

    return (
      <div ref={rowRef} className="relative overflow-hidden" data-testid="swipeable-row-container">
        {/* Reveal layer — visual feedback only, not interactive */}
        {!disabled && (
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 flex items-center justify-end bg-destructive pr-6"
          >
            <Trash2 className="h-5 w-5 text-destructive-foreground" />
          </div>
        )}

        {/* Moving row — slides left to reveal the layer behind it */}
        <animated.div
          {...bind()}
          style={{ x, touchAction: "pan-y", position: "relative" }}
          className="bg-background"
          data-testid="swipeable-row"
        >
          {children}
        </animated.div>
      </div>
    );
  },
);
