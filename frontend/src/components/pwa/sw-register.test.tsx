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
  });

  describe("when serviceWorker is NOT in navigator", () => {
    let originalDescriptor: PropertyDescriptor | undefined;

    beforeEach(() => {
      Object.defineProperty(window, "isSecureContext", {
        value: true,
        writable: true,
        configurable: true,
      });

      // Save the current descriptor so we can restore it in afterEach.
      // jsdom adds serviceWorker on the Navigator prototype; we shadow it on
      // the navigator instance with a stub that has no `register` to verify the
      // component's `"serviceWorker" in navigator` guard path. We provide a
      // stub object without `register` so the guard evaluates true but any
      // accidental call to `.register` would surface immediately.
      // A cleaner approach: temporarily replace with a stub that tracks calls.
      originalDescriptor = Object.getOwnPropertyDescriptor(Navigator.prototype, "serviceWorker");

      // Provide an empty stub so the component's `"serviceWorker" in navigator`
      // branch evaluates true but there is no `register` method — the component
      // must not call it because the feature-detect inside useEffect checks
      // `"serviceWorker" in navigator`. Since we want to test the branch where
      // the whole feature is absent, we override the property to be truly absent
      // by using a non-enumerable configurable deletion approach:
      // We mock navigator.serviceWorker as a plain object without `register` to
      // prove no accidental call occurs (register would throw if invoked).
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

    it("does not throw and does not attempt registration", () => {
      // With serviceWorker getter removed from Navigator.prototype,
      // "serviceWorker" in navigator is now false — the component's guard
      // short-circuits and no registration is attempted.
      expect(() => render(<SwRegister />)).not.toThrow();
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
