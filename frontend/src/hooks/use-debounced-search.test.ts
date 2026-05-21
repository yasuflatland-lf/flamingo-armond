// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useDebouncedSearch } from "./use-debounced-search";

describe("useDebouncedSearch", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  describe("initial state", () => {
    it("returns empty input and null query", () => {
      const { result } = renderHook(() => useDebouncedSearch());

      expect(result.current.input).toBe("");
      expect(result.current.query).toBeNull();
    });
  });

  describe("setInput + debounce", () => {
    it("updates input synchronously but query stays null before delay fires", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("hello");
      });

      expect(result.current.input).toBe("hello");
      expect(result.current.query).toBeNull();
    });

    it("sets query to the trimmed input after delayMs", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("hello");
      });

      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(result.current.query).toBe("hello");
    });

    it("coalesces rapid calls — query is only the final value", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("a");
      });

      act(() => {
        vi.advanceTimersByTime(100);
      });

      // "a" timer has not fired yet
      expect(result.current.query).toBeNull();

      act(() => {
        result.current.setInput("ab");
      });

      act(() => {
        vi.advanceTimersByTime(300);
      });

      // Only the final value should have been committed — never intermediate "a"
      expect(result.current.query).toBe("ab");
    });
  });

  describe("trim behaviour", () => {
    it("sets query to null when input trims to empty", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("   ");
      });

      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(result.current.query).toBeNull();
    });

    it("preserves non-empty content after trimming whitespace", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("  hi  ");
      });

      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(result.current.query).toBe("hi");
    });
  });

  describe("custom delayMs", () => {
    it("does not fire query before custom delay elapses", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch({ delayMs: 500 }));

      act(() => {
        result.current.setInput("x");
      });

      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(result.current.query).toBeNull();

      act(() => {
        vi.advanceTimersByTime(200);
      });

      expect(result.current.query).toBe("x");
    });
  });

  describe("clear()", () => {
    it("immediately resets both input and query, cancelling any pending debounce", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("hello");
      });

      // Advance partway through the debounce window
      act(() => {
        vi.advanceTimersByTime(150);
      });

      expect(result.current.query).toBeNull();

      act(() => {
        result.current.clear();
      });

      expect(result.current.input).toBe("");
      expect(result.current.query).toBeNull();

      // Advance past the original debounce window — no stale timer should fire
      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(result.current.input).toBe("");
      expect(result.current.query).toBeNull();
    });

    it("resets input to empty and query to null after the debounce has already committed", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useDebouncedSearch());

      act(() => {
        result.current.setInput("hello");
      });

      // Advance past the full debounce window so query commits to "hello".
      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(result.current.input).toBe("hello");
      expect(result.current.query).toBe("hello");

      // clear() after commit must reset both synchronously.
      act(() => {
        result.current.clear();
      });

      expect(result.current.input).toBe("");
      expect(result.current.query).toBeNull();
    });
  });
});
