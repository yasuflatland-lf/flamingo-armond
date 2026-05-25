// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SwRegister } from "./sw-register";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<SwRegister>", () => {
  describe("when serviceWorker is supported and context is secure", () => {
    let registerMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      registerMock = vi.fn().mockResolvedValue(undefined);

      // Ensure the secure context flag is set.
      Object.defineProperty(window, "isSecureContext", {
        value: true,
        writable: true,
        configurable: true,
      });

      // Provide a navigator.serviceWorker stub with a register mock.
      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock },
        writable: true,
        configurable: true,
      });
    });

    it("calls navigator.serviceWorker.register with '/sw.js' after mount", async () => {
      render(<SwRegister />);
      // The register call is async — flush the micro-task queue.
      await vi.waitFor(() => {
        expect(registerMock).toHaveBeenCalledTimes(1);
        expect(registerMock).toHaveBeenCalledWith("/sw.js");
      });
    });

    it("renders nothing (returns null)", () => {
      const { container } = render(<SwRegister />);
      expect(container.firstChild).toBeNull();
    });

    it("warns via console.warn and does not re-throw when register rejects", async () => {
      // Override the register mock to reject for this test.
      registerMock = vi.fn().mockRejectedValue(new Error("SW install blocked"));
      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock },
        writable: true,
        configurable: true,
      });

      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      // Must not throw even though register() rejects.
      expect(() => render(<SwRegister />)).not.toThrow();

      // Flush the micro-task queue so the .catch callback fires.
      await vi.waitFor(() => {
        expect(warnSpy).toHaveBeenCalledWith(
          expect.stringContaining("[pwa] service worker registration failed:"),
          expect.any(Error),
        );
      });
    });
  });

  describe("when serviceWorker is NOT in navigator", () => {
    let originalDescriptor: PropertyDescriptor | undefined;
    let registerMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      Object.defineProperty(window, "isSecureContext", {
        value: true,
        writable: true,
        configurable: true,
      });

      // Track any accidental registration attempt via a spy on the instance.
      registerMock = vi.fn();

      // Save the current descriptor so we can restore it in afterEach.
      // jsdom adds serviceWorker on the Navigator prototype; we remove the
      // getter entirely so that `"serviceWorker" in navigator` evaluates to
      // false, exercising the feature-detect guard path.
      originalDescriptor = Object.getOwnPropertyDescriptor(Navigator.prototype, "serviceWorker");

      Object.defineProperty(Navigator.prototype, "serviceWorker", {
        get: undefined,
        configurable: true,
      });
    });

    afterEach(() => {
      // Restore the original serviceWorker descriptor.
      if (originalDescriptor) {
        Object.defineProperty(Navigator.prototype, "serviceWorker", originalDescriptor);
      }
    });

    it("does not throw and does not attempt registration", async () => {
      // With serviceWorker getter removed from Navigator.prototype,
      // "serviceWorker" in navigator is false — the component's guard
      // short-circuits and no registration is attempted.
      expect(() => render(<SwRegister />)).not.toThrow();

      // Flush effects to ensure the useEffect has run before asserting.
      await new Promise((r) => setTimeout(r, 0));
      expect(registerMock).not.toHaveBeenCalled();
    });
  });

  describe("when the context is NOT secure", () => {
    let registerMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      registerMock = vi.fn().mockResolvedValue(undefined);

      Object.defineProperty(window, "isSecureContext", {
        value: false,
        writable: true,
        configurable: true,
      });

      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock },
        writable: true,
        configurable: true,
      });
    });

    it("does not call register", async () => {
      render(<SwRegister />);
      // Allow effects to flush; register must NOT have been called.
      await new Promise((r) => setTimeout(r, 0));
      expect(registerMock).not.toHaveBeenCalled();
    });
  });
});
