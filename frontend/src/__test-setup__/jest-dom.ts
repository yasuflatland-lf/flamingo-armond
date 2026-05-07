// Extend Vitest's expect with @testing-library/jest-dom matchers (toBeInTheDocument, etc.)
// This file is loaded via vitest.config.ts setupFiles for all test environments.
// jest-dom 6+ supports the vitest extend API when globals are available.
import "@testing-library/jest-dom/vitest";
import { Globals } from "@react-spring/web";

Globals.assign({ skipAnimation: true });

// jsdom does not implement window.matchMedia. Components that call
// window.matchMedia (e.g. useReducedMotion, useIsMobile via SidebarProvider)
// fail in jsdom tests without a stub. Provide a minimal non-matching stub so
// every jsdom-environment test has a safe default. Individual tests that need
// to simulate "matches" or "prefers-reduced-motion: reduce" can override the
// stub via Object.defineProperty in their own beforeEach.
if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
      addListener: () => {},
      removeListener: () => {},
    }),
  });
}
