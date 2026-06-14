// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useFitText } from "./use-fit-text";

// Attaches the hook's ref to a real <p> so the effect's measure path runs.
// jsdom has no layout engine, so clientWidth/scrollWidth read 0 and the hook
// returns maxPx unchanged — this exercises the hook's CONTROL FLOW (ref attach,
// measure-at-zero, ResizeObserver guard), not the numeric layout math (that is
// covered by computeFitFontSize in use-fit-text.test.ts).
function Harness({ text, max, min }: { text: string; max: number; min: number }) {
  const { ref, fontPx } = useFitText<HTMLParagraphElement>(text, max, min);
  return (
    <p ref={ref} data-testid="fit" style={{ fontSize: `${fontPx}px` }}>
      {text}
    </p>
  );
}

describe("useFitText — ResizeObserver absent (jsdom default)", () => {
  it("initializes fontPx to maxPx and does not throw when ResizeObserver is undefined", () => {
    // jsdom provides no ResizeObserver, so this reaches the typeof-undefined
    // guard. A throw here would blank the card on first paint.
    expect(() => render(<Harness text="hello" max={48} min={20} />)).not.toThrow();
    expect(screen.getByTestId("fit")).toHaveStyle({ fontSize: "48px" });
  });
});

describe("useFitText — ResizeObserver present", () => {
  let observed: Element[];
  let disconnectCount: number;

  // Captures observe targets and disconnect calls. The constructor callback is
  // intentionally ignored — these tests assert wiring (which element is
  // observed, disconnect on unmount), not re-fit behavior.
  class FakeResizeObserver {
    observe(el: Element) {
      observed.push(el);
    }
    unobserve() {}
    disconnect() {
      disconnectCount += 1;
    }
  }

  beforeEach(() => {
    observed = [];
    disconnectCount = 0;
    vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("observes the PARENT element, not the text element, to avoid a measure loop", () => {
    render(
      <div data-testid="parent">
        <Harness text="hello" max={48} min={20} />
      </div>,
    );

    const textEl = screen.getByTestId("fit");
    const parentEl = screen.getByTestId("parent");
    expect(observed).toContain(parentEl);
    expect(observed).not.toContain(textEl);
  });

  it("disconnects the observer on unmount (no leak)", () => {
    const { unmount } = render(<Harness text="hello" max={48} min={20} />);
    expect(disconnectCount).toBe(0);

    unmount();

    expect(disconnectCount).toBeGreaterThan(0);
  });
});
