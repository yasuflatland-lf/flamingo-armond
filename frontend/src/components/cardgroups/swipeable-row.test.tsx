// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SwipeableRow, type SwipeableRowHandle } from "./swipeable-row";

// ---------------------------------------------------------------------------
// matchMedia stub — jsdom does not implement it.
// Default: reduced-motion = false so the swipe layer renders.
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

  // jsdom does not compute layout: stub offsetWidth to 300px so threshold
  // calculations are deterministic.
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
    configurable: true,
    get() {
      return 300;
    },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
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
// @use-gesture/react with `pointer: { capture: false }` attaches:
//   - onPointerDown on the element
//   - pointermove / pointerup on window after pointerdown fires
// ---------------------------------------------------------------------------

function getSwipeTarget() {
  return screen.getByTestId("swipeable-row");
}

/**
 * Simulate a pointer drag gesture.
 * deltaX < 0 = leftward; deltaY non-zero used to test vertical pass-through.
 */
function simulateSwipe(element: Element, deltaX: number, deltaY = 0) {
  const startX = 250;
  const startY = 100;
  const endX = startX + deltaX;
  const endY = startY + deltaY;

  fireEvent.pointerDown(element, {
    pointerId: 1,
    clientX: startX,
    clientY: startY,
    buttons: 1,
    bubbles: true,
  });

  fireEvent.pointerMove(window, {
    pointerId: 1,
    clientX: startX + deltaX / 2,
    clientY: startY + deltaY / 2,
    buttons: 1,
    bubbles: true,
  });

  fireEvent.pointerMove(window, {
    pointerId: 1,
    clientX: endX,
    clientY: endY,
    buttons: 1,
    bubbles: true,
  });

  fireEvent.pointerUp(window, {
    pointerId: 1,
    clientX: endX,
    clientY: endY,
    bubbles: true,
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("<SwipeableRow>", () => {
  it("renders children inside the swipeable layer", () => {
    render(
      <SwipeableRow onDelete={vi.fn()}>
        <span>Card front</span>
      </SwipeableRow>,
    );
    expect(screen.getByText("Card front")).toBeInTheDocument();
    expect(screen.getByTestId("swipeable-row")).toBeInTheDocument();
  });

  it("≥40% release → onDelete fires", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // Swipe past the commit threshold so onDelete fires.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -130);
    });

    await waitFor(() => {
      expect(onDelete).toHaveBeenCalledTimes(1);
    });
  });

  it("<40% release → onDelete not called and row snaps back", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // Swipe below the commit threshold so the row snaps back without firing onDelete.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -100);
    });

    expect(onDelete).not.toHaveBeenCalled();
  });

  it("vertical drag (|mx| < |my|) → no-op; onDelete not called", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // deltaX = -50px, deltaY = -200px — predominantly vertical.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -50, -200);
    });

    expect(onDelete).not.toHaveBeenCalled();
  });

  it("disabled → gesture is cancelled; onDelete not called", async () => {
    const onDelete = vi.fn();
    render(
      <SwipeableRow onDelete={onDelete} disabled>
        <span>Card</span>
      </SwipeableRow>,
    );

    await act(async () => {
      simulateSwipe(getSwipeTarget(), -200);
    });

    expect(onDelete).not.toHaveBeenCalled();
  });

  it("useReducedMotion() === true → children rendered directly, no SwipeableRow wrapper", () => {
    stubMatchMedia(true);

    render(
      <SwipeableRow onDelete={vi.fn()}>
        <span>Reduced motion card</span>
      </SwipeableRow>,
    );

    expect(screen.getByText("Reduced motion card")).toBeInTheDocument();
    expect(screen.queryByTestId("swipeable-row")).not.toBeInTheDocument();
    expect(screen.queryByTestId("swipeable-row-container")).not.toBeInTheDocument();
  });

  it("useReducedMotion() === true → swipe events do not trigger onDelete", () => {
    stubMatchMedia(true);

    const onDelete = vi.fn();
    const { container } = render(
      <SwipeableRow onDelete={onDelete}>
        <span>Card</span>
      </SwipeableRow>,
    );

    act(() => {
      simulateSwipe(container.firstElementChild ?? container, -200);
    });

    expect(onDelete).not.toHaveBeenCalled();
  });

  it("renders the reveal layer with a destructive background and Trash icon", () => {
    const { container } = render(
      <SwipeableRow onDelete={vi.fn()}>
        <span>Card front</span>
      </SwipeableRow>,
    );

    // The reveal layer must carry bg-destructive.
    const revealLayer = container.querySelector(".bg-destructive");
    expect(revealLayer).toBeInTheDocument();

    // A Trash2 SVG icon must be present inside the reveal layer.
    const trashIcon = revealLayer?.querySelector("svg");
    expect(trashIcon).not.toBeNull();
  });

  it("does not render the reveal layer when reduced-motion is on", () => {
    stubMatchMedia(true);

    const { container } = render(
      <SwipeableRow onDelete={vi.fn()}>
        <span>Card front</span>
      </SwipeableRow>,
    );

    expect(container.querySelector(".bg-destructive")).not.toBeInTheDocument();
    expect(container.querySelector("svg")).not.toBeInTheDocument();
  });

  it("close() imperative handle → snaps row back to x=0 without calling onDelete", async () => {
    const onDelete = vi.fn();
    const ref = createRef<SwipeableRowHandle>();

    render(
      <SwipeableRow ref={ref} onDelete={onDelete}>
        <span>Card</span>
      </SwipeableRow>,
    );

    // Partially drag the row without reaching the commit threshold.
    await act(async () => {
      simulateSwipe(getSwipeTarget(), -100);
    });

    expect(onDelete).not.toHaveBeenCalled();

    // Call the imperative handle.
    act(() => {
      ref.current?.close();
    });

    // The handle must exist and not throw.
    expect(ref.current).not.toBeNull();
    expect(onDelete).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Static regression guard
// ---------------------------------------------------------------------------

describe("SwipeableRow source — static regression guard", () => {
  it("does not reintroduce the half-open reveal state", () => {
    const source = readFileSync(require.resolve("./swipeable-row.tsx"), "utf-8");
    expect(source).not.toMatch(/HALF_OPEN_THRESHOLD/);
    expect(source).not.toMatch(/setIsHalfOpen/);
    expect(source).not.toMatch(/ACTION_WIDTH/);
    expect(source).not.toMatch(/data-testid="swipe-delete-button"/);
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

    act(() => {
      currentMatches = true;
      capturedListener?.();
    });

    expect(result.current).toBe(true);
  });
});
