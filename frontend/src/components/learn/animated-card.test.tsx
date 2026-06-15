// @vitest-environment jsdom

import { Controller } from "@react-spring/web";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createRef, StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AnimatedCard, type AnimatedCardHandle } from "./animated-card";
import type { SwipeCardData } from "./swipe-card";

// ---------------------------------------------------------------------------
// matchMedia stub — jsdom does not implement it. Default reduced-motion=false
// so the spring + gesture layer renders. window.innerWidth/innerHeight drive
// the fly-off distance, so give them deterministic non-zero values.
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
  Object.defineProperty(window, "innerWidth", { configurable: true, value: 1024 });
  Object.defineProperty(window, "innerHeight", { configurable: true, value: 768 });
});

afterEach(() => {
  vi.restoreAllMocks();
});

const CARD: SwipeCardData = {
  id: "card-1",
  front: "Hello",
  back: "Hola",
  cefrLevel: null,
  userCardState: { due: "2026-04-30T00:00:00Z", state: 0 },
  cardgroupId: "cg-1",
};

function getCard() {
  return screen.getByTestId("swipe-card");
}

function revealedProps() {
  return { revealed: true, onReveal: vi.fn() };
}

/**
 * Spy on every `Controller.start` call (the underlying driver behind the
 * `api.start` the component issues) and record the target `x` and whether a
 * `config` was supplied. The fly-off is the only call that carries a `config`
 * AND drives `x` to `±innerWidth`; the spring-reset / drag-track `else` branch
 * issues calls with no `config`. This lets a jsdom test observe the exact
 * mechanism of the regression — jsdom settles the spring synchronously and so
 * cannot reproduce the `finished: false` drop directly.
 */
function spyControllerStart() {
  const calls: Array<{ x: unknown; hasConfig: boolean }> = [];
  const original = Controller.prototype.start;
  vi.spyOn(Controller.prototype, "start").mockImplementation(function (
    this: Controller,
    ...args: Parameters<typeof original>
  ) {
    const [props] = args;
    if (props && typeof props === "object" && !Array.isArray(props)) {
      // The controller's props type is intentionally loose; we only read the
      // transform target `x` and whether a `config` (the fly-off marker) is set.
      const record = props as { x?: unknown; config?: unknown };
      calls.push({ x: record.x, hasConfig: Boolean(record.config) });
    }
    return original.apply(this, args);
  });
  return calls;
}

/**
 * Simulate a committing rightward drag past the 160px horizontal-commit
 * threshold. @use-gesture with `pointer: { capture: false }` attaches the
 * pointerdown on the element and pointermove/pointerup on window. The terminal
 * (active=false) frame is the one that fires the fly-off.
 */
function fireCommittingSwipeRight(element: Element) {
  const startX = 100;
  const startY = 300;
  fireEvent.pointerDown(element, {
    pointerId: 1,
    clientX: startX,
    clientY: startY,
    buttons: 1,
    bubbles: true,
  });
  for (const dx of [40, 120, 220]) {
    fireEvent.pointerMove(window, {
      pointerId: 1,
      clientX: startX + dx,
      clientY: startY,
      buttons: 1,
      bubbles: true,
    });
  }
  fireEvent.pointerUp(window, {
    pointerId: 1,
    clientX: startX + 220,
    clientY: startY,
    bubbles: true,
  });
}

function fireCommittingSwipeDown(element: Element) {
  const startX = 100;
  const startY = 100;
  fireEvent.pointerDown(element, {
    pointerId: 1,
    clientX: startX,
    clientY: startY,
    buttons: 1,
    bubbles: true,
  });
  for (const dy of [40, 120, 220]) {
    fireEvent.pointerMove(window, {
      pointerId: 1,
      clientX: startX,
      clientY: startY + dy,
      buttons: 1,
      bubbles: true,
    });
  }
  fireEvent.pointerUp(window, {
    pointerId: 1,
    clientX: startX,
    clientY: startY + 220,
    bubbles: true,
  });
}

/** A fresh re-grab gesture (a different pointerId) on the same element. */
function fireReGrab(element: Element) {
  fireEvent.pointerDown(element, {
    pointerId: 2,
    clientX: 500,
    clientY: 300,
    buttons: 1,
    bubbles: true,
  });
  fireEvent.pointerMove(window, {
    pointerId: 2,
    clientX: 520,
    clientY: 300,
    buttons: 1,
    bubbles: true,
  });
  fireEvent.pointerUp(window, {
    pointerId: 2,
    clientX: 520,
    clientY: 300,
    bubbles: true,
  });
}

describe("<AnimatedCard> — front-only reveal phase", () => {
  it("clicking the front-only active card reveals without rating", () => {
    const onReveal = vi.fn();
    const onSwipe = vi.fn();
    render(
      <AnimatedCard card={CARD} isActive revealed={false} onReveal={onReveal} onSwipe={onSwipe} />,
    );

    fireEvent.click(getCard());

    expect(onReveal).toHaveBeenCalledTimes(1);
    expect(onSwipe).not.toHaveBeenCalled();
  });

  it("allows rating swipes while the active card is front-only (reveal is optional)", async () => {
    // Reveal no longer gates rating: a committing swipe on a front-only card
    // flies it off and commits, exactly as it does once revealed.
    const onSwipe = vi.fn();
    render(
      <AnimatedCard card={CARD} isActive revealed={false} onReveal={vi.fn()} onSwipe={onSwipe} />,
    );

    await act(async () => {
      fireCommittingSwipeDown(getCard());
    });

    await waitFor(
      () => {
        expect(onSwipe).toHaveBeenCalledTimes(1);
      },
      { timeout: 2000 },
    );
    expect(onSwipe).toHaveBeenCalledWith(CARD, "down");
  });
});

describe("<AnimatedCard> — committing gesture commits exactly once", () => {
  it("a committing rightward swipe fires onSwipe exactly once with the right direction", async () => {
    const onSwipe = vi.fn();
    render(<AnimatedCard card={CARD} isActive onSwipe={onSwipe} {...revealedProps()} />);

    await act(async () => {
      fireCommittingSwipeRight(getCard());
    });

    await waitFor(
      () => {
        expect(onSwipe).toHaveBeenCalledTimes(1);
      },
      { timeout: 2000 },
    );
    expect(onSwipe).toHaveBeenCalledWith(CARD, "right");
  });

  it("does not commit when the swipe falls short of the threshold (snaps back)", async () => {
    const onSwipe = vi.fn();
    render(<AnimatedCard card={CARD} isActive onSwipe={onSwipe} {...revealedProps()} />);

    await act(async () => {
      const el = getCard();
      fireEvent.pointerDown(el, {
        pointerId: 1,
        clientX: 100,
        clientY: 300,
        buttons: 1,
        bubbles: true,
      });
      // Release under HORIZONTAL_FLICK_MIN_PX (40px): synchronous fireEvent yields a
      // huge synthetic velocity, so any larger drag would trip the flick-commit path.
      fireEvent.pointerMove(window, {
        pointerId: 1,
        clientX: 118,
        clientY: 300,
        buttons: 1,
        bubbles: true,
      });
      fireEvent.pointerMove(window, {
        pointerId: 1,
        clientX: 130,
        clientY: 300,
        buttons: 1,
        bubbles: true,
      });
      fireEvent.pointerUp(window, {
        pointerId: 1,
        clientX: 130,
        clientY: 300,
        bubbles: true,
      });
    });

    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(onSwipe).not.toHaveBeenCalled();
  });
});

describe("<AnimatedCard> — re-grab during fly-off must not interrupt the exit", () => {
  // Root-cause regression. The user-reported bug: after a committing swipe
  // starts the fly-off, the finger never lifts, so @use-gesture begins a fresh
  // gesture during the ~200 ms fly-off. Pre-fix, that gesture fell through to
  // the spring-reset `else` and issued `api.start({ x: 0 })`, interrupting the
  // fly-off — the card snapped back to centre AND the fly-off resolved
  // `finished: false`, dropping the commit. jsdom settles springs
  // synchronously, so the dropped-commit symptom cannot be observed directly;
  // the interrupting `api.start({ x: 0 })` call IS the mechanism and is
  // deterministic. Assert that no spring-reset call is issued after the fly-off.
  it("issues no spring-reset api.start after the fly-off begins", async () => {
    const calls = spyControllerStart();
    const onSwipe = vi.fn();
    render(<AnimatedCard card={CARD} isActive onSwipe={onSwipe} {...revealedProps()} />);

    // The committing swipe AND the re-grab both fire synchronously inside this
    // act block — the interrupting spring-reset (if the guard were missing)
    // would be recorded here, independent of how long the spring takes to
    // settle. This makes the core assertion below timing-independent.
    act(() => {
      fireCommittingSwipeRight(getCard());
      // Re-grab synchronously, before the fly-off has settled.
      fireReGrab(getCard());
    });

    // Locate the fly-off call: the only one carrying a duration `config`. The
    // drag-track / spring-reset `else` calls never carry a config. (Using the
    // config marker rather than a hard-coded x makes this direction-robust.)
    const flyOffIndex = calls.findIndex((call) => call.hasConfig);
    expect(flyOffIndex).toBeGreaterThanOrEqual(0);

    // After the fly-off, no spring-reset `x: 0` may be issued — that is the
    // interruption that strands the card. Pre-fix this slice contains a
    // trailing `{ x: 0 }`; post-fix the re-grab is bailed before the `else`.
    const afterFlyOff = calls.slice(flyOffIndex + 1);
    expect(afterFlyOff.some((call) => call.x === 0)).toBe(false);

    // And the commit still fires exactly once (poll: the spring settles on its
    // own timeline, which can stretch under parallel test load).
    await waitFor(
      () => {
        expect(onSwipe).toHaveBeenCalledTimes(1);
      },
      { timeout: 2000 },
    );
    expect(onSwipe).toHaveBeenCalledWith(CARD, "right");
  });
});

describe("<AnimatedCard> — a re-render must not re-apply the spring initializer", () => {
  // Root-cause regression for the real-browser fly-off-snap-back bug. With the
  // bare `useSpring(() => …)` function form, react-spring leaves the
  // controller's `ctrl.ref` unset, so its per-commit layout effect re-applies
  // the initializer props (`x: 0`, default config) on EVERY render via
  // `Controller.start`. While the card rests at centre that is a silent no-op,
  // but a render landing mid-fly-off (the release frame re-renders the parent
  // stack through `onSwipeProgress(null, 0)`) re-targets the in-flight spring
  // back to `x: 0`: the card snaps to centre, the fly-off resolves
  // `finished: false`, the deferred commit is dropped, and `exitingRef` latches
  // so every later gesture bails. Attaching an explicit `useSpringRef` makes
  // the layout effect QUEUE the initializer instead of starting it.
  //
  // This is timing-independent in jsdom: the layout effect re-applies on every
  // commit regardless of spring state. The initializer re-apply (the bug) is the
  // only config-carrying start that ALSO drives the transform key `x` (to 0). The
  // reveal flip's `api.start` also carries a config (its ease-in `duration`) but
  // targets `rotateY` only — no `x` — so the `x !== undefined` discriminator
  // excludes it. Pre-fix the initializer re-apply (config + `x: 0`) is present on
  // mount and every re-render; post-fix the only `x`-driving starts come from a
  // gesture / fly-off, neither of which a resting re-render triggers.
  it("issues no initializer re-apply (config-carrying start) when a resting card re-renders", () => {
    const calls = spyControllerStart();
    const onSwipe = vi.fn();
    const { rerender } = render(
      <AnimatedCard card={CARD} isActive onSwipe={onSwipe} {...revealedProps()} />,
    );
    // Force a fresh commit with no gesture and no flyOut (new onSwipe identity).
    rerender(<AnimatedCard card={CARD} isActive onSwipe={vi.fn()} {...revealedProps()} />);
    // A config-carrying start that ALSO drives `x` is the initializer re-apply.
    // The reveal flip carries a config but no `x`, so it is correctly excluded.
    expect(calls.some((call) => call.hasConfig && call.x !== undefined)).toBe(false);
  });
});

describe("<AnimatedCard> — reveal flip is a half-turn with ease-in", () => {
  // The reveal flip rotates the card a HALF-TURN (rotateY ±180) and overrides the
  // controller's spring config with a fixed-duration ease-in (easeInQuart) so the
  // rotation starts slow and snaps at the end. The SIGN of FLIP_DEGREES is the
  // visual direction knob (tuned in the browser), so this test pins the half-turn
  // + ease-in mechanism, NOT the specific direction. The spring initializer also
  // names rotateY but carries `{ tension, friction }` (no `duration`), so the
  // `duration` check isolates the flip from the initializer re-apply.
  it("drives the reveal flip a half-turn (±180) carrying a fixed-duration ease-in", () => {
    const flipCalls: Array<{ rotateY: unknown; hasDuration: boolean }> = [];
    const original = Controller.prototype.start;
    vi.spyOn(Controller.prototype, "start").mockImplementation(function (
      this: Controller,
      ...args: Parameters<typeof original>
    ) {
      const [props] = args;
      if (props && typeof props === "object" && !Array.isArray(props)) {
        const record = props as { rotateY?: unknown; config?: { duration?: unknown } };
        if ("rotateY" in record) {
          flipCalls.push({
            rotateY: record.rotateY,
            hasDuration: Boolean(record.config?.duration),
          });
        }
      }
      return original.apply(this, args);
    });

    render(<AnimatedCard card={CARD} isActive onSwipe={vi.fn()} {...revealedProps()} />);

    // A half-turn (±180) start carrying an ease-in duration config is the flip.
    expect(
      flipCalls.some((call) => (call.rotateY === 180 || call.rotateY === -180) && call.hasDuration),
    ).toBe(true);
  });
});

describe("<AnimatedCard> — fly-off spring rejection still commits", () => {
  it("commits anyway (and warns) when the fly-off spring rejects on a teardown race", async () => {
    // The animation is cosmetic; the commit is the load-bearing side effect.
    // If the fly-off spring rejects (a frozen SpringValue mid-animation), the
    // deferred chain must still commit so the card is never stranded.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const original = Controller.prototype.start;
    vi.spyOn(Controller.prototype, "start").mockImplementation(function (
      this: Controller,
      ...args: Parameters<typeof original>
    ) {
      const [props] = args;
      // Reject only the fly-off (the duration-config call), not the per-frame
      // drag-track / setup calls.
      if (props && typeof props === "object" && !Array.isArray(props) && "config" in props) {
        return Promise.reject(new Error("frozen SpringValue")) as ReturnType<typeof original>;
      }
      return original.apply(this, args);
    });

    const onSwipe = vi.fn();
    const handleRef = createRef<AnimatedCardHandle | null>();
    render(
      <AnimatedCard
        card={CARD}
        isActive
        onSwipe={onSwipe}
        handleRef={handleRef}
        {...revealedProps()}
      />,
    );

    act(() => {
      handleRef.current?.flyOut("right");
    });

    await waitFor(
      () => {
        expect(onSwipe).toHaveBeenCalledTimes(1);
      },
      { timeout: 2000 },
    );
    expect(onSwipe).toHaveBeenCalledWith(CARD, "right");
    expect(warnSpy).toHaveBeenCalled();
  });
});

describe("<AnimatedCard> — reduced motion", () => {
  it("renders the revealed face without a rotateY rotator", () => {
    stubMatchMedia(true);
    const onSwipe = vi.fn();

    render(
      <AnimatedCard card={CARD} isActive revealed={true} onReveal={vi.fn()} onSwipe={onSwipe} />,
    );

    expect(screen.queryByTestId("swipe-card-rotator")).not.toBeInTheDocument();
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("commits synchronously without driving a fly-off spring", () => {
    stubMatchMedia(true);
    const calls = spyControllerStart();
    const onSwipe = vi.fn();
    const handleRef = createRef<AnimatedCardHandle | null>();

    render(
      <AnimatedCard
        card={CARD}
        isActive
        onSwipe={onSwipe}
        handleRef={handleRef}
        {...revealedProps()}
      />,
    );

    act(() => {
      handleRef.current?.flyOut("left");
    });

    expect(onSwipe).toHaveBeenCalledTimes(1);
    expect(onSwipe).toHaveBeenCalledWith(CARD, "left");
    // No fly-off spring is driven on the reduced-motion path: there is no
    // duration-config start call.
    expect(calls.some((call) => call.hasConfig && call.x !== undefined)).toBe(false);
  });
});

describe("<AnimatedCard> — double-exit guard", () => {
  it("a second flyOut before the first settles does not double-commit", async () => {
    const onSwipe = vi.fn();
    const handleRef = createRef<AnimatedCardHandle | null>();

    render(
      <AnimatedCard
        card={CARD}
        isActive
        onSwipe={onSwipe}
        handleRef={handleRef}
        {...revealedProps()}
      />,
    );

    act(() => {
      handleRef.current?.flyOut("right");
      // Second trigger while the first fly-off is still in flight — dropped by
      // the exitingRef guard inside runExit.
      handleRef.current?.flyOut("left");
    });

    await waitFor(
      () => {
        expect(onSwipe).toHaveBeenCalledTimes(1);
      },
      { timeout: 2000 },
    );
    expect(onSwipe).toHaveBeenCalledWith(CARD, "right");
  });
});

describe("<AnimatedCard> — React StrictMode double-mount", () => {
  // Regression guard for the mountedRef StrictMode bug.
  //
  // React StrictMode (enabled in dev via next.config reactStrictMode: true)
  // deliberately runs the mount sequence setup → cleanup → setup for every
  // effect. The cleanup sets mountedRef.current = false. Without an explicit
  // reset in the effect body, the second setup leaves mountedRef.current ===
  // false for the live component, so the deferred fly-off commit is silently
  // dropped and onSwipe is never called. The fix resets mountedRef.current =
  // true in the effect body so both the StrictMode re-mount and a genuine
  // remount restore the flag before the component is interactive.
  it("fires onSwipe exactly once after a committing swipe when wrapped in StrictMode", async () => {
    const onSwipe = vi.fn();

    render(
      <StrictMode>
        <AnimatedCard card={CARD} isActive onSwipe={onSwipe} {...revealedProps()} />
      </StrictMode>,
    );

    await act(async () => {
      fireCommittingSwipeRight(getCard());
    });

    await waitFor(
      () => {
        expect(onSwipe).toHaveBeenCalledTimes(1);
      },
      { timeout: 2000 },
    );
    expect(onSwipe).toHaveBeenCalledWith(CARD, "right");
  });
});

describe("<AnimatedCard> — CEFR badge does not duplicate across the flipped faces", () => {
  // Regression for the mirrored top-left badge bug. In the non-reduced-motion
  // reveal flip the card renders a front face (revealed=false) AND a back face
  // (revealed=true, rotateY 180). When the badge lived inside CardContent, BOTH
  // faces carried a CefrBadge; after the half-turn flip the front face is turned
  // away (rotateY -180 = horizontally mirrored) and its badge leaked through
  // `[backface-visibility:hidden]` — an absolutely-positioned, z-indexed child
  // forms its own stacking context that the ancestor's backface-visibility does
  // not clip in WebKit — surfacing as a mirrored badge in the TOP-LEFT corner.
  // The fix lifts the badge OUT of the flip rotator, so exactly one badge
  // renders regardless of flip state.
  it("renders exactly one CEFR badge when revealed (no mirrored backface duplicate)", () => {
    render(
      <AnimatedCard
        card={{ ...CARD, cefrLevel: "C2" }}
        isActive
        onSwipe={vi.fn()}
        {...revealedProps()}
      />,
    );

    expect(screen.getAllByLabelText("CEFR level C2")).toHaveLength(1);
  });

  it("renders the badge once even when the card is front-only (not yet revealed)", () => {
    render(
      <AnimatedCard
        card={{ ...CARD, cefrLevel: "C2" }}
        isActive
        revealed={false}
        onReveal={vi.fn()}
        onSwipe={vi.fn()}
      />,
    );

    expect(screen.getAllByLabelText("CEFR level C2")).toHaveLength(1);
  });

  it("renders no CEFR badge when the card has no level (null)", () => {
    render(
      <AnimatedCard
        card={{ ...CARD, cefrLevel: null }}
        isActive
        onSwipe={vi.fn()}
        {...revealedProps()}
      />,
    );

    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
  });
});
