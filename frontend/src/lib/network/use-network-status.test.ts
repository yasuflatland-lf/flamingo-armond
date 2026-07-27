// @vitest-environment happy-dom

import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getServerSnapshot, useIsOffline } from "./use-network-status";

const originalOnLine = Object.getOwnPropertyDescriptor(window.navigator, "onLine");

function setOnLine(value: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    value,
    configurable: true,
  });
}

afterEach(() => {
  if (originalOnLine) {
    Object.defineProperty(window.navigator, "onLine", originalOnLine);
  } else {
    Reflect.deleteProperty(window.navigator, "onLine");
  }
  vi.restoreAllMocks();
});

describe("useIsOffline", () => {
  it("uses false for the server and initial hydration snapshot", () => {
    setOnLine(false);

    expect(getServerSnapshot()).toBe(false);
  });

  it("reports true after mount when navigator.onLine is false", () => {
    setOnLine(false);

    const { result } = renderHook(() => useIsOffline());

    expect(result.current).toBe(true);
  });

  it("updates when offline and online events fire", () => {
    setOnLine(true);
    const { result } = renderHook(() => useIsOffline());

    act(() => {
      setOnLine(false);
      window.dispatchEvent(new Event("offline"));
    });
    expect(result.current).toBe(true);

    act(() => {
      setOnLine(true);
      window.dispatchEvent(new Event("online"));
    });
    expect(result.current).toBe(false);
  });

  it("removes both connectivity listeners on unmount", () => {
    const addEventListener = vi.spyOn(window, "addEventListener");
    const removeEventListener = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderHook(() => useIsOffline());
    const onlineHandler = addEventListener.mock.calls.find(([event]) => event === "online")?.[1];
    const offlineHandler = addEventListener.mock.calls.find(([event]) => event === "offline")?.[1];

    expect(onlineHandler).toEqual(expect.any(Function));
    expect(offlineHandler).toEqual(expect.any(Function));

    unmount();

    expect(removeEventListener).toHaveBeenCalledWith("online", onlineHandler);
    expect(removeEventListener).toHaveBeenCalledWith("offline", offlineHandler);
  });
});
