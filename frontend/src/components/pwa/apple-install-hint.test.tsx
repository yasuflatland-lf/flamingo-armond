// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppleInstallHint } from "./apple-install-hint";

const DISMISSED_KEY = "pwa-ios-install-hint-dismissed";

// jsdom provides a minimal localStorage; clear it between tests.
afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

// Helper: mock navigator.userAgent to a given string.
function mockUserAgent(ua: string) {
  Object.defineProperty(navigator, "userAgent", {
    value: ua,
    writable: true,
    configurable: true,
  });
}

// Helper: mock window.matchMedia to return a given matches value.
function mockMatchMedia(matches: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches,
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

describe("<AppleInstallHint>", () => {
  describe("on an iOS device, non-standalone, not dismissed", () => {
    beforeEach(() => {
      mockUserAgent(
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
      );
      mockMatchMedia(false); // display-mode: standalone not active
      // Ensure navigator.standalone is falsy (jsdom default).
      Object.defineProperty(navigator, "standalone", {
        value: undefined,
        writable: true,
        configurable: true,
      });
      localStorage.removeItem(DISMISSED_KEY);
    });

    it("renders the install banner with 'Add to Home Screen' copy", async () => {
      render(<AppleInstallHint />);
      // The banner is revealed by a useEffect; wait for the state update.
      const hint = await screen.findByLabelText("Install hint");
      expect(hint).toBeInTheDocument();
      expect(screen.getByText(/Add to Home Screen/i)).toBeInTheDocument();
    });

    it("renders the 'Install flamingo' heading", async () => {
      render(<AppleInstallHint />);
      await screen.findByText("Install flamingo");
    });

    it("renders a dismiss button", async () => {
      render(<AppleInstallHint />);
      const btn = await screen.findByRole("button", {
        name: "Dismiss install hint",
      });
      expect(btn).toBeInTheDocument();
    });

    it("hides the banner and sets localStorage when dismiss is clicked", async () => {
      const user = userEvent.setup();
      render(<AppleInstallHint />);

      const btn = await screen.findByRole("button", {
        name: "Dismiss install hint",
      });
      await user.click(btn);

      // Banner should be gone from the DOM.
      expect(screen.queryByLabelText("Install hint")).not.toBeInTheDocument();

      // Dismissal must be persisted so the banner stays hidden on next visit.
      expect(localStorage.getItem(DISMISSED_KEY)).toBe("true");
    });
  });

  describe("on a non-iOS UA", () => {
    beforeEach(() => {
      mockUserAgent(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
      );
      mockMatchMedia(false);
      localStorage.removeItem(DISMISSED_KEY);
    });

    it("does not render the install banner", async () => {
      render(<AppleInstallHint />);
      // Give effects time to run, then assert nothing appeared.
      await new Promise((r) => setTimeout(r, 0));
      expect(screen.queryByLabelText("Install hint")).not.toBeInTheDocument();
    });
  });

  describe("when display-mode is standalone (already installed)", () => {
    beforeEach(() => {
      mockUserAgent(
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
      );
      // Simulate the app running in standalone mode.
      mockMatchMedia(true);
      localStorage.removeItem(DISMISSED_KEY);
    });

    it("does not render the install banner", async () => {
      render(<AppleInstallHint />);
      await new Promise((r) => setTimeout(r, 0));
      expect(screen.queryByLabelText("Install hint")).not.toBeInTheDocument();
    });
  });

  describe("when the user has already dismissed the hint", () => {
    beforeEach(() => {
      mockUserAgent(
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
      );
      mockMatchMedia(false);
      localStorage.setItem(DISMISSED_KEY, "true");
    });

    it("does not render the install banner", async () => {
      render(<AppleInstallHint />);
      await new Promise((r) => setTimeout(r, 0));
      expect(screen.queryByLabelText("Install hint")).not.toBeInTheDocument();
    });
  });
});
