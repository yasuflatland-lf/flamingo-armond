// @vitest-environment jsdom
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SwipeableRow, type SwipeableRowHandle } from "./swipeable-row";

// ---------------------------------------------------------------------------
// matchMedia stub — jsdom does not implement it.
// The default stub reports reduced-motion = false so the swipe layer renders.
// Individual tests override `matches` to exercise the reduced-motion path.
// ---------------------------------------------------------------------------

function stubMatchMedia(reducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query.includes("prefers-reduced-motion") ? reducedMotion : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

beforeEach(() => {
  stubMatchMedia(false);

  // jsdom does not compute layout: make offsetWidth return a stable 300px so
  // threshold calculations (60% = 180px, 30% = 90px) are deterministic.
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
    configurable: true,
    get() {
      return 300;
    },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  // Reset offsetWidth to default (undefined) to avoid cross-test pollution.
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
    configurable: true,
    get() {
      return 0;
    },
  });
});

// ---------------------------------------------------------------------------
// Pointer-event helpers
//
// @use-gesture/react spreads onPointerDown / onPointerMove / onPointerUp on
// the animated element when `pointer.capture = false`. jsdom forwards these as
// React synthetic pointer events, so fireEvent.pointerXxx works.
//
// Row width is stubbed to 300px.
//   Full swipe  ≥ 60% → 180px
//   Half swipe  30-59% → 90–179px
// ---------------------------------------------------------------------------

/** Returns the animated div that @use-gesture binds its handlers to. */
function getSwipeTarget() {
  return screen.getByTestId("swipeable-row");
}

/**
 * Simulate a pointer drag gesture.
 *
 * @use-gesture/react with `pointer: { capture: false }` (our config) attaches:
 *   - onPointerDown → bound to the element via React props
 *   - pointermove / pointerup → added to `window` after pointerdown fires
 *
 * Therefore:
 *   - pointerdown is fired on the element
 *   - pointermove and pointerup are fired on window
 */
function simulateSwipe(element: Element, deltaX: number) {
  const startX = 250;
  const endX = startX + deltaX;

  // Step 1: pointerdown on the element — triggers setupPointer() which adds
  // pointermove/pointerup listeners to window.
  fireEvent.pointerDown(element, {
    pointerId: 1,
    clientX: startX,
    clientY: 100,
    buttons: 1,
    bubbles: true,
  });

  // Step 2: pointermove on window — use-gesture reads clientX from the event.
  fireEvent.pointerMove(window, {
    pointerId: 1,
    clientX: startX + deltaX / 2,
    clientY: 100,
    buttons: 1,
    bubbles: true,
  });

  fireEvent.pointerMove(window, {
    pointerId: 1,
    clientX: endX,
    clientY: 100,
    buttons: 1,
    bubbles: true,
  });

  // Step 3: pointerup on window — triggers the `last: true` callback branch.
  fireEvent.pointerUp(window, {
    pointerId: 1,
    clientX: endX,
    clientY: 100,
    bubbles: true,
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("<SwipeableRow>", () => {
  it("T1 — renders children inside the swipeable layer", () => {
    render(
      <SwipeableRow onDelete={vi.fn()} ariaLabel={null}>
        <span>Card front</span>
      </SwipeableRow>,
    );
    expect(screen.getByText("Card front")).toBeInTheDocument();
    expect(screen.getByTestId("swipeable-row")).toBeInTheDocument();
  });

  it("T2 — full swipe left (≥ 60% of width = 180px) calls onDelete", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} ariaLabel={null}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // 200px left swipe — above 180px (60%) threshold.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -200);
    });

    await waitFor(() => {
      expect(onDelete).toHaveBeenCalledTimes(1);
    });
  });

  it("T3 — half-swipe (30%–60% = 90–179px) reveals the Delete button without calling onDelete", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} ariaLabel="Delete card">
        <span>Card</span>
      </SwipeableRow>,
    );

    // 100px left swipe — between 90px (30%) and 180px (60%).
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -100);
    });

    // onDelete must NOT have been called yet.
    expect(onDelete).not.toHaveBeenCalled();

    // The Delete button should now be tappable (tabIndex 0).
    const deleteBtn = screen.getByTestId("swipe-delete-button");
    expect(deleteBtn).toBeInTheDocument();
    expect(deleteBtn).toHaveAttribute("tabIndex", "0");
  });

  it("T4 — tapping the revealed Delete button after a half-swipe calls onDelete", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} ariaLabel="Delete card">
        <span>Card</span>
      </SwipeableRow>,
    );

    // Half-open the row.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -100);
    });

    // Tap the revealed Delete button.
    await act(async () => {
      fireEvent.click(screen.getByTestId("swipe-delete-button"));
    });

    await waitFor(() => {
      expect(onDelete).toHaveBeenCalledTimes(1);
    });
  });

  it("T5 — short swipe (< 30% = 90px) snaps back, Delete button is hidden (tabIndex -1)", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} ariaLabel={null}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // 60px left swipe — below 90px (30%) threshold.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -60);
    });

    expect(onDelete).not.toHaveBeenCalled();

    const deleteBtn = screen.getByTestId("swipe-delete-button");
    // After snap-back, button should not be keyboard-reachable.
    expect(deleteBtn).toHaveAttribute("tabIndex", "-1");
  });

  it("T6 — rightward swipe does nothing (unidirectional left-only)", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} ariaLabel={null}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // Positive deltaX = rightward movement.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), 200);
    });

    expect(onDelete).not.toHaveBeenCalled();
    const deleteBtn = screen.getByTestId("swipe-delete-button");
    expect(deleteBtn).toHaveAttribute("tabIndex", "-1");
  });

  it("T7 — disabled=true makes swipe a no-op (onDelete not called, tabIndex stays -1)", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} disabled ariaLabel={null}>
        <span>Card</span>
      </SwipeableRow>,
    );

    await act(async () => {
      simulateSwipe(getSwipeTarget(), -200);
    });

    expect(onDelete).not.toHaveBeenCalled();
    const deleteBtn = screen.getByTestId("swipe-delete-button");
    expect(deleteBtn).toHaveAttribute("tabIndex", "-1");
  });

  it("T8 — prefers-reduced-motion: reduce → swipe layer NOT rendered, children rendered directly", () => {
    stubMatchMedia(true);

    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} ariaLabel={null}>
        <span>Reduced motion card</span>
      </SwipeableRow>,
    );

    // Children must still be visible.
    expect(screen.getByText("Reduced motion card")).toBeInTheDocument();

    // Swipe layer and action button must NOT be present.
    expect(screen.queryByTestId("swipeable-row")).not.toBeInTheDocument();
    expect(screen.queryByTestId("swipeable-row-container")).not.toBeInTheDocument();
    expect(screen.queryByTestId("swipe-delete-button")).not.toBeInTheDocument();
  });

  it("T9 — prefers-reduced-motion: reduce → simulating swipe events does not trigger onDelete", () => {
    stubMatchMedia(true);

    const onDelete = vi.fn();
    const { container } = render(
      <SwipeableRow onDelete={onDelete} ariaLabel={null}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // There is no swipe target to interact with; confirm nothing reachable.
    expect(screen.queryByTestId("swipeable-row")).not.toBeInTheDocument();
    expect(onDelete).not.toHaveBeenCalled();

    // Try firing events on the container root (no gesture layer attached).
    act(() => {
      simulateSwipe(container.firstElementChild ?? container, -200);
    });

    expect(onDelete).not.toHaveBeenCalled();
  });

  it("T10 — imperative close() ref snaps a half-open row back to resting state", async () => {
    const onDelete = vi.fn();
    const ref = createRef<SwipeableRowHandle>();

    render(
      <SwipeableRow ref={ref} onDelete={onDelete} ariaLabel={null}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // Half-open the row first.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -100);
    });

    // Button should be half-open (tabIndex 0).
    expect(screen.getByTestId("swipe-delete-button")).toHaveAttribute("tabIndex", "0");

    // Close programmatically (simulating an outside-row tap from the parent).
    act(() => {
      ref.current?.close();
    });

    // After close, button should no longer be keyboard-reachable.
    await waitFor(() => {
      expect(screen.getByTestId("swipe-delete-button")).toHaveAttribute("tabIndex", "-1");
    });
  });
});

// ---------------------------------------------------------------------------
// useReducedMotion hook tests
// ---------------------------------------------------------------------------

import { renderHook } from "@testing-library/react";
import { useReducedMotion } from "@/lib/use-reduced-motion";

describe("useReducedMotion", () => {
  it("returns false when prefers-reduced-motion does not match", () => {
    stubMatchMedia(false);
    const { result } = renderHook(() => useReducedMotion());
    expect(result.current).toBe(false);
  });

  it("returns true when prefers-reduced-motion: reduce is active", () => {
    stubMatchMedia(true);
    const { result } = renderHook(() => useReducedMotion());
    expect(result.current).toBe(true);
  });

  it("updates reactively when the media query changes", async () => {
    // useSyncExternalStore: the subscribe callback is () => void (not a
    // MediaQueryListEvent handler). After React calls it, useSyncExternalStore
    // re-invokes getSnapshot() — which reads window.matchMedia(...).matches —
    // so the mock must return the updated matches value by that time.
    let capturedListener: (() => void) | null = null;
    let currentMatches = false;

    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        get matches() {
          return query.includes("prefers-reduced-motion") ? currentMatches : false;
        },
        media: query,
        onchange: null,
        addEventListener: (_event: string, handler: () => void) => {
          if (query.includes("prefers-reduced-motion")) {
            capturedListener = handler;
          }
        },
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    const { result } = renderHook(() => useReducedMotion());
    expect(result.current).toBe(false);

    // Simulate the user enabling reduced-motion: update matches, then notify.
    act(() => {
      currentMatches = true;
      capturedListener?.();
    });

    expect(result.current).toBe(true);
  });
});
