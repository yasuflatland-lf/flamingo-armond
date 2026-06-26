// @vitest-environment happy-dom
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useBulkSelection } from "./use-bulk-selection";

describe("useBulkSelection", () => {
  describe("initial state", () => {
    it("returns an empty Set and count 0", () => {
      const { result } = renderHook(() => useBulkSelection());

      expect(result.current.selectedIds).toEqual(new Set());
      expect(result.current.count).toBe(0);
    });
  });

  describe("toggleSelected", () => {
    it("adds an id to selectedIds", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
      });

      expect(result.current.selectedIds).toEqual(new Set(["a"]));
      expect(result.current.count).toBe(1);
      expect(result.current.isSelected("a")).toBe(true);
    });

    it("removes an id when called twice", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
      });
      expect(result.current.count).toBe(1);

      act(() => {
        result.current.toggleSelected("a");
      });
      expect(result.current.selectedIds).toEqual(new Set());
      expect(result.current.count).toBe(0);
      expect(result.current.isSelected("a")).toBe(false);
    });

    it("handles multiple ids", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
        result.current.toggleSelected("b");
        result.current.toggleSelected("c");
      });

      expect(result.current.count).toBe(3);
      expect(result.current.isSelected("a")).toBe(true);
      expect(result.current.isSelected("b")).toBe(true);
      expect(result.current.isSelected("c")).toBe(true);
    });
  });

  describe("clearSelection", () => {
    it("resets selectedIds to empty Set", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
        result.current.toggleSelected("b");
      });
      expect(result.current.count).toBe(2);

      act(() => {
        result.current.clearSelection();
      });

      expect(result.current.selectedIds).toEqual(new Set());
      expect(result.current.count).toBe(0);
    });
  });

  describe("isSelected", () => {
    it("returns true for selected ids", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
      });

      expect(result.current.isSelected("a")).toBe(true);
      expect(result.current.isSelected("b")).toBe(false);
    });
  });

  describe("selectedIds immutability", () => {
    it("returns a new Set identity on each toggle", () => {
      const { result } = renderHook(() => useBulkSelection());

      const before = result.current.selectedIds;
      act(() => {
        result.current.toggleSelected("a");
      });
      const after = result.current.selectedIds;

      expect(before).not.toBe(after);
      expect(before).toEqual(new Set());
      expect(after).toEqual(new Set(["a"]));
    });

    it("returns a new Set identity on clearSelection", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
      });
      const before = result.current.selectedIds;

      act(() => {
        result.current.clearSelection();
      });
      const after = result.current.selectedIds;

      expect(before).not.toBe(after);
    });
  });

  describe("callback stability", () => {
    it("maintains stable callback identities across renders", () => {
      const { result, rerender } = renderHook(() => useBulkSelection());

      const toggleSelectedBefore = result.current.toggleSelected;
      const clearSelectionBefore = result.current.clearSelection;

      rerender();

      expect(result.current.toggleSelected).toBe(toggleSelectedBefore);
      expect(result.current.clearSelection).toBe(clearSelectionBefore);
    });

    it("isSelected identity changes when selectedIds changes", () => {
      const { result } = renderHook(() => useBulkSelection());

      const isSelectedBefore = result.current.isSelected;

      act(() => {
        result.current.toggleSelected("a");
      });

      const isSelectedAfter = result.current.isSelected;

      // isSelected depends on selectedIds, so it changes when selectedIds changes
      expect(isSelectedBefore).not.toBe(isSelectedAfter);
      // But the result is the same
      expect(isSelectedAfter("a")).toBe(true);
    });
  });

  describe("generic over TId", () => {
    it("works with number type parameter", () => {
      const { result } = renderHook(() => useBulkSelection<number>());

      act(() => {
        result.current.toggleSelected(1);
        result.current.toggleSelected(2);
      });

      expect(result.current.count).toBe(2);
      expect(result.current.isSelected(1)).toBe(true);
      expect(result.current.isSelected(2)).toBe(true);
      expect(result.current.isSelected(3)).toBe(false);
    });
  });

  describe("count derivation", () => {
    it("updates count when toggleSelected changes selectedIds", () => {
      const { result } = renderHook(() => useBulkSelection());

      expect(result.current.count).toBe(0);

      act(() => {
        result.current.toggleSelected("a");
      });
      expect(result.current.count).toBe(1);

      act(() => {
        result.current.toggleSelected("b");
      });
      expect(result.current.count).toBe(2);

      act(() => {
        result.current.toggleSelected("a");
      });
      expect(result.current.count).toBe(1);
    });

    it("updates count when clearSelection is called", () => {
      const { result } = renderHook(() => useBulkSelection());

      act(() => {
        result.current.toggleSelected("a");
        result.current.toggleSelected("b");
      });
      expect(result.current.count).toBe(2);

      act(() => {
        result.current.clearSelection();
      });
      expect(result.current.count).toBe(0);
    });
  });
});
