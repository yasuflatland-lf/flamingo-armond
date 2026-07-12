// @vitest-environment happy-dom
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useSheetTargetLoading } from "./use-sheet-target-loading";

describe("useSheetTargetLoading", () => {
  it("is not loading and does not match when no sheet is open", () => {
    const { result } = renderHook(() =>
      useSheetTargetLoading(null, { called: false, variables: undefined, loading: false }),
    );

    expect(result.current).toEqual({ matchesSheet: false, loading: false });
  });

  it("gates as loading on the first render before the firing effect runs", () => {
    // URL just landed on ?edit=<id>: called is still false, no variables yet.
    const { result } = renderHook(() =>
      useSheetTargetLoading("user-1", { called: false, variables: undefined, loading: false }),
    );

    expect(result.current).toEqual({ matchesSheet: false, loading: true });
  });

  it("gates as loading while the query's variables still target a previous id", () => {
    // Switched from ?edit=user-1 to ?edit=user-2 before the first query settled.
    const { result } = renderHook(() =>
      useSheetTargetLoading("user-2", {
        called: true,
        variables: { id: "user-1" },
        loading: false,
      }),
    );

    expect(result.current).toEqual({ matchesSheet: false, loading: true });
  });

  it("resolves once the settled result matches the open sheet id", () => {
    const { result } = renderHook(() =>
      useSheetTargetLoading("user-1", {
        called: true,
        variables: { id: "user-1" },
        loading: false,
      }),
    );

    expect(result.current).toEqual({ matchesSheet: true, loading: false });
  });

  it("stays loading while the query itself is in flight, even for a matching id", () => {
    const { result } = renderHook(() =>
      useSheetTargetLoading("role-1", {
        called: true,
        variables: { id: "role-1" },
        loading: true,
      }),
    );

    expect(result.current).toEqual({ matchesSheet: true, loading: true });
  });

  it("uses strict equality: a numeric variable id does not match a string sheet id", () => {
    // Generated query variables type ids as `string | number`; the gate uses
    // strict `===`, so a numeric `7` never matches the string sheet id `"7"`.
    const { result } = renderHook(() =>
      useSheetTargetLoading("7", { called: true, variables: { id: 7 }, loading: false }),
    );

    expect(result.current).toEqual({ matchesSheet: false, loading: true });
  });
});
