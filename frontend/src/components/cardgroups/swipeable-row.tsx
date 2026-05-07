"use client";

import { animated, useSpring } from "@react-spring/web";
import { useDrag } from "@use-gesture/react";
import { Trash2 } from "lucide-react";
import { forwardRef, useCallback, useImperativeHandle, useRef, useState } from "react";
import { useReducedMotion } from "@/lib/use-reduced-motion";

// ---------------------------------------------------------------------------
// Public API types
// ---------------------------------------------------------------------------

export interface SwipeableRowHandle {
  /** Programmatically close a half-open row (snap back to resting position). */
  close(): void;
}

export interface SwipeableRowProps {
  /** Row content — the card front/back text, checkboxes, etc. */
  children: React.ReactNode;
  /** Fires when the user commits a delete (full-swipe or revealed-button tap). */
  onDelete: () => void;
  /**
   * When true, swipe gesture is completely disabled so the selection-mode
   * checkboxes receive touch events without interference.
   */
  disabled?: boolean;
  /**
   * Accessible label for the delete action button revealed on half-swipe.
   * Required (`string | null`) — pass `null` to accept the default "Delete"
   * label, or a contextual string (e.g. "Delete card") to override it.
   * See `.claude/rules/frontend-typescript-conventions.md` § "Required
   * `string | null` over optional `?: string | null`".
   */
  ariaLabel: string | null;
}

// ---------------------------------------------------------------------------
// Thresholds
// ---------------------------------------------------------------------------

/** Fraction of row width at which a released swipe snaps to half-open. */
const HALF_OPEN_THRESHOLD = 0.3;

/** Fraction of row width at which a swipe is treated as a full delete commit. */
const FULL_SWIPE_THRESHOLD = 0.6;

/** Width of the revealed Delete button action area in pixels. */
const ACTION_WIDTH = 80;

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * Unidirectional (left-only) swipe-to-delete row.
 *
 * Behaviour:
 * - Swipe left >= 60% of row width → row slides off screen → `onDelete()` fires.
 * - Swipe left 30%–60% then release → snaps to half-open; the Delete button is
 *   revealed. Tapping that button calls `onDelete()`.
 * - Swipe in any other direction → no-op, snaps back.
 * - `disabled` → gesture is a no-op; touch events pass through to children.
 * - `prefers-reduced-motion: reduce` → swipe layer not rendered; children are
 *   returned as-is. The parent is responsible for the delete affordance in that
 *   case (e.g. the hover Delete icon is always-visible on reduced-motion).
 *
 * The component exposes an imperative `close()` handle via `ref` so a parent
 * list can close any half-open row when the user taps a different row.
 */
export const SwipeableRow = forwardRef<SwipeableRowHandle, SwipeableRowProps>(function SwipeableRow(
  { children, onDelete, disabled = false, ariaLabel },
  ref,
) {
  const reducedMotion = useReducedMotion();

  // When reduced-motion is requested, render children without any swipe layer.
  // The parent's hover Delete icon serves as the delete affordance.
  if (reducedMotion) {
    return <>{children}</>;
  }

  return (
    <SwipeableRowInner ref={ref} onDelete={onDelete} disabled={disabled} ariaLabel={ariaLabel}>
      {children}
    </SwipeableRowInner>
  );
});

// ---------------------------------------------------------------------------
// Inner animated implementation (only mounted when motion is allowed)
// ---------------------------------------------------------------------------

const SwipeableRowInner = forwardRef<SwipeableRowHandle, SwipeableRowProps>(
  function SwipeableRowInner({ children, onDelete, disabled = false, ariaLabel }, ref) {
    /** Whether the row is currently snapped to the half-open (reveal) state. */
    const [isHalfOpen, setIsHalfOpen] = useState(false);

    /** Ref to the outer row element, used to read its rendered width. */
    const rowRef = useRef<HTMLDivElement>(null);

    const [{ x }, api] = useSpring(() => ({
      x: 0,
      config: { tension: 520, friction: 38 },
    }));

    const startDeleteAnimation = useCallback(
      (onComplete: () => void) => {
        const rowWidth = rowRef.current?.offsetWidth ?? 300;
        // SpringRef.start() returns an array of Promises (one per spring key).
        // Promise.all resolves once all keys have settled.
        Promise.all(api.start({ x: -rowWidth }))
          .then(() => {
            setIsHalfOpen(false);
            onComplete();
          })
          .catch(() => {
            // Animation was interrupted (e.g. component unmounted); no-op.
          });
      },
      [api],
    );

    // Expose close() to parent list so it can close this row when another row
    // is tapped.
    useImperativeHandle(ref, () => ({
      close() {
        setIsHalfOpen(false);
        api.start({ x: 0 });
      },
    }));

    const bind = useDrag(
      ({ active, movement: [mx, my], last, cancel }) => {
        if (disabled) {
          cancel();
          return;
        }

        // Ignore predominantly vertical movements so the user can still scroll
        // the list. A gesture is treated as horizontal if |dx| >= |dy|.
        const isHorizontal = Math.abs(mx) >= Math.abs(my);
        if (!isHorizontal && !last) {
          // Vertical drag in progress — do not interfere with scroll.
          return;
        }

        // Clamp to leftward movement only (unidirectional).
        const clampedMx = Math.min(mx, 0);

        const rowWidth = rowRef.current?.offsetWidth ?? 300;
        const fraction = Math.abs(clampedMx) / rowWidth;

        if (last) {
          if (isHorizontal && fraction >= FULL_SWIPE_THRESHOLD) {
            // Full swipe: fly off to the left, then fire onDelete.
            startDeleteAnimation(onDelete);
          } else if (isHorizontal && fraction >= HALF_OPEN_THRESHOLD) {
            // Half-swipe: snap to reveal position.
            setIsHalfOpen(true);
            api.start({ x: -ACTION_WIDTH });
          } else {
            // Not far enough or vertical: snap back.
            setIsHalfOpen(false);
            api.start({ x: 0 });
          }
          return;
        }

        // During drag: follow the finger, clamped to leftward movement only.
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

    const handleDeleteButtonClick = () => {
      startDeleteAnimation(onDelete);
    };

    return (
      <div ref={rowRef} className="relative overflow-hidden" data-testid="swipeable-row-container">
        {/* Delete action revealed behind the row */}
        <div
          className="absolute inset-y-0 right-0 flex items-center justify-center bg-destructive"
          style={{ width: ACTION_WIDTH }}
          aria-hidden={!isHalfOpen}
        >
          <button
            type="button"
            className="flex h-full w-full items-center justify-center text-destructive-foreground"
            aria-label={ariaLabel === null ? "Delete" : ariaLabel}
            tabIndex={isHalfOpen ? 0 : -1}
            onClick={handleDeleteButtonClick}
            data-testid="swipe-delete-button"
          >
            <Trash2 className="h-5 w-5" aria-hidden="true" />
          </button>
        </div>

        {/* Swipeable foreground row */}
        <animated.div
          {...bind()}
          style={{ x, touchAction: "pan-y" }}
          className="relative bg-background"
          data-testid="swipeable-row"
        >
          {children}
        </animated.div>
      </div>
    );
  },
);
