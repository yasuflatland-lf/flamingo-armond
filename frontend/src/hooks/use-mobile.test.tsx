// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useIsMobile } from "./use-mobile";

// Helpers to configure the jsdom matchMedia stub and window.innerWidth.

function stubMatchMedia(matches: boolean) {
  const listeners = new Set<() => void>();

  const mql = {
    matches,
    media: "",
    onchange: null,
    addEventListener: vi.fn((_event: string, cb: () => void) => {
      listeners.add(cb);
    }),
    removeEventListener: vi.fn((_event: string, cb: () => void) => {
      listeners.delete(cb);
    }),
    dispatchEvent: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
  };

  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: vi.fn().mockReturnValue(mql),
  });

  return {
    mql,
    triggerChange: () =>
      listeners.forEach((cb) => {
        cb();
      }),
  };
}

function setInnerWidth(width: number) {
  Object.defineProperty(window, "innerWidth", {
    writable: true,
    configurable: true,
    value: width,
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useIsMobile", () => {
  describe("when window.innerWidth is below the mobile breakpoint (768)", () => {
    beforeEach(() => {
      setInnerWidth(375);
      stubMatchMedia(true);
    });

    it("returns true", () => {
      const { result } = renderHook(() => useIsMobile());
      expect(result.current).toBe(true);
    });
  });

  describe("when window.innerWidth is at or above the mobile breakpoint (768)", () => {
    beforeEach(() => {
      setInnerWidth(1024);
      stubMatchMedia(false);
    });

    it("returns false", () => {
      const { result } = renderHook(() => useIsMobile());
      expect(result.current).toBe(false);
    });
  });

  describe("reactivity — updates when the media query fires a change event", () => {
    it("flips from desktop to mobile when window.innerWidth narrows", async () => {
      setInnerWidth(1024);
      const { triggerChange } = stubMatchMedia(false);

      const { result, rerender } = renderHook(() => useIsMobile());
      expect(result.current).toBe(false);

      // Simulate a resize into mobile territory.
      setInnerWidth(375);
      act(() => {
        triggerChange();
      });
      rerender();

      expect(result.current).toBe(true);
    });
  });

  describe("does not cause extra renders on mount", () => {
    it("renders exactly once on mount", () => {
      setInnerWidth(1024);
      stubMatchMedia(false);

      let renderCount = 0;
      const { result } = renderHook(() => {
        renderCount += 1;
        return useIsMobile();
      });

      // useSyncExternalStore must not trigger a second synchronous render.
      expect(renderCount).toBe(1);
      expect(result.current).toBe(false);
    });
  });

  describe("subscribe/unsubscribe lifecycle", () => {
    it("removes the matchMedia listener on unmount", () => {
      setInnerWidth(1024);
      const { mql } = stubMatchMedia(false);

      const { unmount } = renderHook(() => useIsMobile());
      expect(mql.removeEventListener).not.toHaveBeenCalled();

      unmount();
      expect(mql.removeEventListener).toHaveBeenCalledTimes(1);
    });
  });
});
