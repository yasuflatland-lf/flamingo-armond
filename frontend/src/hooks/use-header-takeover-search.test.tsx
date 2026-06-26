// @vitest-environment happy-dom
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useHeaderTakeoverSearch } from "./use-header-takeover-search";

type SearchStateDetail = { active: boolean; visible: boolean };

// Capture every flamingo:search-state detail the hook dispatches.
function listenSearchState() {
  const calls: SearchStateDetail[] = [];
  function onState(e: Event) {
    calls.push((e as CustomEvent<SearchStateDetail>).detail);
  }
  window.addEventListener("flamingo:search-state", onState);
  return {
    calls,
    stop: () => window.removeEventListener("flamingo:search-state", onState),
  };
}

describe("useHeaderTakeoverSearch", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  describe("initial state", () => {
    it("starts closed and dispatches search-state {active:false, visible:false} on mount", () => {
      const state = listenSearchState();
      const { result } = renderHook(() => useHeaderTakeoverSearch());

      expect(result.current.searchOpen).toBe(false);
      expect(state.calls).toContainEqual({ active: false, visible: false });
      state.stop();
    });
  });

  describe("flamingo:open-search", () => {
    it("opens the takeover when the header trigger dispatches the event", () => {
      const { result } = renderHook(() => useHeaderTakeoverSearch());

      act(() => {
        window.dispatchEvent(new CustomEvent("flamingo:open-search"));
      });

      expect(result.current.searchOpen).toBe(true);
    });
  });

  describe("search-state reporting", () => {
    it("dispatches {active:true} after a non-empty query debounces", () => {
      vi.useFakeTimers();
      const state = listenSearchState();
      const { result } = renderHook(() => useHeaderTakeoverSearch());

      act(() => {
        result.current.setInput("hello");
      });
      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(state.calls).toContainEqual({ active: true, visible: false });
      state.stop();
    });

    it("reports visible:true while the bar is open", () => {
      const state = listenSearchState();
      renderHook(() => useHeaderTakeoverSearch());

      act(() => {
        window.dispatchEvent(new CustomEvent("flamingo:open-search"));
      });

      expect(state.calls).toContainEqual({ active: false, visible: true });
      state.stop();
    });
  });

  describe("closeSearch", () => {
    it("closes the takeover", () => {
      const { result } = renderHook(() => useHeaderTakeoverSearch());

      act(() => {
        window.dispatchEvent(new CustomEvent("flamingo:open-search"));
      });
      expect(result.current.searchOpen).toBe(true);

      act(() => {
        result.current.closeSearch();
      });
      expect(result.current.searchOpen).toBe(false);
    });
  });

  describe("unmount", () => {
    it("resets the header trigger with {active:false, visible:false}", () => {
      const { unmount } = renderHook(() => useHeaderTakeoverSearch());
      const state = listenSearchState();

      act(() => {
        unmount();
      });

      expect(state.calls).toContainEqual({ active: false, visible: false });
      state.stop();
    });
  });
});
