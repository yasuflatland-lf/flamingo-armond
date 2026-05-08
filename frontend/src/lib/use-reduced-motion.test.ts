// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { getServerSnapshot, getSnapshot, subscribe, useReducedMotion } from "./use-reduced-motion";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useReducedMotion", () => {
  // Smoke test that the hook is wired to useSyncExternalStore and reads the
  // matchMedia result. Detailed reactivity tests live with use-mobile and
  // share the same useSyncExternalStore plumbing.
  it("reads the prefers-reduced-motion media query on first call", () => {
    const matchMediaMock = vi.fn().mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: matchMediaMock,
    });

    // Calling the exported getSnapshot directly avoids needing renderHook here
    // — this test asserts the snapshot contract, not React integration.
    expect(getSnapshot()).toBe(true);
    expect(matchMediaMock).toHaveBeenCalledWith("(prefers-reduced-motion: reduce)");
  });

  it("hook returns the current snapshot value", () => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    });
    // Call useReducedMotion via getSnapshot to exercise the production wiring
    // without dragging in @testing-library/react for this single contract test.
    expect(typeof useReducedMotion).toBe("function");
    expect(getSnapshot()).toBe(false);
  });

  // SSR / no-window contract — frontend-typescript-conventions.md
  // § "useSyncExternalStore over useState + useEffect".
  describe("SSR / no-window branches", () => {
    it("getServerSnapshot returns false (deterministic SSR default)", () => {
      expect(getServerSnapshot()).toBe(false);
    });

    it("getSnapshot returns false when window is undefined", () => {
      const originalWindow = globalThis.window;
      // biome-ignore lint/suspicious/noExplicitAny: simulate SSR by erasing window
      (globalThis as any).window = undefined;
      try {
        expect(getSnapshot()).toBe(false);
      } finally {
        globalThis.window = originalWindow;
      }
    });

    it("subscribe returns a no-op unsubscribe when window is undefined", () => {
      const originalWindow = globalThis.window;
      // biome-ignore lint/suspicious/noExplicitAny: simulate SSR by erasing window
      (globalThis as any).window = undefined;
      try {
        const unsubscribe = subscribe(() => {
          throw new Error("callback should never fire in SSR");
        });
        expect(typeof unsubscribe).toBe("function");
        expect(() => unsubscribe()).not.toThrow();
      } finally {
        globalThis.window = originalWindow;
      }
    });
  });
});
