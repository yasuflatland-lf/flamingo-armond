// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useFitText } from "./use-fit-text";

// Attaches the hook's ref to a real <p> so the effect's measure path runs.
// jsdom has no layout engine, so clientWidth/scrollWidth read 0 unless a test
// stubs them — this exercises the hook's CONTROL FLOW (ref attach, measure,
// ResizeObserver guard, re-fit on resize), while the numeric layout math is
// covered by computeFitFontSize in use-fit-text.test.ts.
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
  let capturedCallback: ResizeObserverCallback | null;

  // Captures the observe targets, disconnect calls, and the resize callback so a
  // test can fire a synthetic resize. The constructor records the callback
  // (real ResizeObserver delivers it); the body is never auto-invoked by jsdom.
  class FakeResizeObserver {
    constructor(cb: ResizeObserverCallback) {
      capturedCallback = cb;
    }
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
    capturedCallback = null;
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

  it("re-fits (shrinks) when an observed resize reports a now-overflowing width", () => {
    render(<Harness text="cardiovascular" max={48} min={20} />);
    const el = screen.getByTestId("fit");
    // Initial: jsdom widths are 0, so the unmeasured guard keeps maxPx.
    expect(el).toHaveStyle({ fontSize: "48px" });

    // Simulate a layout where the single word is twice the available width.
    Object.defineProperty(el, "clientWidth", { configurable: true, value: 100 });
    Object.defineProperty(el, "scrollWidth", { configurable: true, value: 200 });

    // Fire the resize the observer is subscribed to.
    act(() => {
      capturedCallback?.([], {} as ResizeObserver);
    });

    // 48 * 100 / 200 = 24 — the re-fit ran and updated the applied size.
    expect(el).toHaveStyle({ fontSize: "24px" });
  });
});
