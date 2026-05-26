// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SwRegister } from "./sw-register";

// Flush all pending microtasks so .then/.catch callbacks fire before asserting.
const flushMicrotasks = () => new Promise((r) => setTimeout(r, 0));

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
});

describe("<SwRegister>", () => {
  // ------------------------------------------------------------------ //
  // Production + secure context — happy path                            //
  // ------------------------------------------------------------------ //
  describe("production + secure context", () => {
    let registerMock: ReturnType<typeof vi.fn>;
    let getRegistrationsMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      vi.stubEnv("NODE_ENV", "production");

      registerMock = vi.fn().mockResolvedValue(undefined);
      getRegistrationsMock = vi.fn().mockResolvedValue([]);

      Object.defineProperty(window, "isSecureContext", {
        value: true,
        writable: true,
        configurable: true,
      });

      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock, getRegistrations: getRegistrationsMock },
        writable: true,
        configurable: true,
      });
    });

    it("calls navigator.serviceWorker.register with '/sw.js' after mount", async () => {
      render(<SwRegister />);
      await vi.waitFor(() => {
        expect(registerMock).toHaveBeenCalledTimes(1);
        expect(registerMock).toHaveBeenCalledWith("/sw.js");
      });
    });

    it("does NOT call getRegistrations (no unregister in production)", async () => {
      render(<SwRegister />);
      await flushMicrotasks();
      expect(getRegistrationsMock).not.toHaveBeenCalled();
    });

    it("renders nothing (returns null)", () => {
      const { container } = render(<SwRegister />);
      expect(container.firstChild).toBeNull();
    });

    it("warns via console.warn and does not re-throw when register rejects", async () => {
      registerMock = vi.fn().mockRejectedValue(new Error("SW install blocked"));
      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock, getRegistrations: getRegistrationsMock },
        writable: true,
        configurable: true,
      });

      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      expect(() => render(<SwRegister />)).not.toThrow();

      await vi.waitFor(() => {
        expect(warnSpy).toHaveBeenCalledWith(
          expect.stringContaining("[pwa] service worker registration failed:"),
          expect.any(Error),
        );
      });
    });
  });

  // ------------------------------------------------------------------ //
  // Production + insecure context — register must be skipped            //
  // ------------------------------------------------------------------ //
  describe("production + insecure context", () => {
    let registerMock: ReturnType<typeof vi.fn>;
    let getRegistrationsMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      vi.stubEnv("NODE_ENV", "production");

      registerMock = vi.fn().mockResolvedValue(undefined);
      getRegistrationsMock = vi.fn().mockResolvedValue([]);

      Object.defineProperty(window, "isSecureContext", {
        value: false,
        writable: true,
        configurable: true,
      });

      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock, getRegistrations: getRegistrationsMock },
        writable: true,
        configurable: true,
      });
    });

    it("does not call register when isSecureContext is false", async () => {
      render(<SwRegister />);
      await flushMicrotasks();
      expect(registerMock).not.toHaveBeenCalled();
    });
  });

  // ------------------------------------------------------------------ //
  // Development (default Vitest NODE_ENV = "test") — cleanup branch     //
  // ------------------------------------------------------------------ //
  describe("development / non-production environment", () => {
    let registerMock: ReturnType<typeof vi.fn>;
    let getRegistrationsMock: ReturnType<typeof vi.fn>;
    let fakeRegistrations: { unregister: ReturnType<typeof vi.fn> }[];

    beforeEach(() => {
      // NODE_ENV defaults to "test" in Vitest — no stubEnv needed, but we
      // explicitly confirm by NOT stubbing to "production".

      registerMock = vi.fn().mockResolvedValue(undefined);

      fakeRegistrations = [
        { unregister: vi.fn().mockResolvedValue(true) },
        { unregister: vi.fn().mockResolvedValue(true) },
      ];
      getRegistrationsMock = vi.fn().mockResolvedValue(fakeRegistrations);

      Object.defineProperty(window, "isSecureContext", {
        value: true,
        writable: true,
        configurable: true,
      });

      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock, getRegistrations: getRegistrationsMock },
        writable: true,
        configurable: true,
      });
    });

    it("does NOT call navigator.serviceWorker.register", async () => {
      render(<SwRegister />);
      await flushMicrotasks();
      expect(registerMock).not.toHaveBeenCalled();
    });

    it("calls getRegistrations to clean up leftover service workers", async () => {
      render(<SwRegister />);
      await vi.waitFor(() => {
        expect(getRegistrationsMock).toHaveBeenCalledTimes(1);
      });
    });

    it("calls unregister on every returned registration", async () => {
      render(<SwRegister />);
      await vi.waitFor(() => {
        for (const reg of fakeRegistrations) {
          expect(reg.unregister).toHaveBeenCalledTimes(1);
        }
      });
    });

    it("warns via console.warn and does not throw when getRegistrations rejects", async () => {
      const cleanupError = new Error("getRegistrations unavailable");
      getRegistrationsMock = vi.fn().mockRejectedValue(cleanupError);
      Object.defineProperty(navigator, "serviceWorker", {
        value: { register: registerMock, getRegistrations: getRegistrationsMock },
        writable: true,
        configurable: true,
      });

      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      expect(() => render(<SwRegister />)).not.toThrow();

      await vi.waitFor(() => {
        expect(warnSpy).toHaveBeenCalledWith(
          expect.stringContaining("[pwa] service worker cleanup failed:"),
          cleanupError,
        );
      });
    });
  });

  // ------------------------------------------------------------------ //
  // Unsupported browser — "serviceWorker" not in navigator              //
  // ------------------------------------------------------------------ //
  describe("when serviceWorker is NOT in navigator", () => {
    let originalDescriptor: PropertyDescriptor | undefined;
    let registerMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      registerMock = vi.fn();

      Object.defineProperty(window, "isSecureContext", {
        value: true,
        writable: true,
        configurable: true,
      });

      // jsdom adds serviceWorker on Navigator.prototype; remove the getter so
      // that "serviceWorker" in navigator evaluates to false.
      originalDescriptor = Object.getOwnPropertyDescriptor(Navigator.prototype, "serviceWorker");

      Object.defineProperty(Navigator.prototype, "serviceWorker", {
        get: undefined,
        configurable: true,
      });
    });

    afterEach(() => {
      if (originalDescriptor) {
        Object.defineProperty(Navigator.prototype, "serviceWorker", originalDescriptor);
      }
    });

    it("does not throw and does not attempt registration or cleanup", async () => {
      expect(() => render(<SwRegister />)).not.toThrow();
      await flushMicrotasks();
      expect(registerMock).not.toHaveBeenCalled();
    });
  });
});
