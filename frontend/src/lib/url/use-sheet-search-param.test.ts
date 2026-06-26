// @vitest-environment happy-dom
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSheetSearchParam } from "./use-sheet-search-param";

const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRefresh = vi.fn();

let mockPathname = "/admin/roles";
let mockSearchParamsValue = "";

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
    replace: mockReplace,
  }),
  useSearchParams: () => new URLSearchParams(mockSearchParamsValue),
}));

describe("useSheetSearchParam", () => {
  beforeEach(() => {
    mockPush.mockReset();
    mockReplace.mockReset();
    mockRefresh.mockReset();
    mockPathname = "/admin/roles";
    mockSearchParamsValue = "";
  });

  it("returns closed when no sheet query param is present", () => {
    mockSearchParamsValue = "filter=active";

    const { result } = renderHook(() => useSheetSearchParam());

    expect(result.current.state).toEqual({ mode: "closed" });
  });

  it("returns new when ?new=true is present", () => {
    mockSearchParamsValue = "new=true&filter=active";

    const { result } = renderHook(() => useSheetSearchParam());

    expect(result.current.state).toEqual({ mode: "new" });
  });

  it("returns new when both sheet params are present", () => {
    mockSearchParamsValue = "new=true&edit=role-1";

    const { result } = renderHook(() => useSheetSearchParam());

    expect(result.current.state).toEqual({ mode: "new" });
  });

  it("returns edit with the selected id when ?edit=<id> is present", () => {
    mockSearchParamsValue = "edit=role-1";

    const { result } = renderHook(() => useSheetSearchParam());

    expect(result.current.state).toEqual({ mode: "edit", id: "role-1" });
  });

  it("pushes ?new=true while preserving unrelated query params and removing edit", () => {
    mockSearchParamsValue = "filter=active&edit=role-1&page=2";
    const { result } = renderHook(() => useSheetSearchParam());

    act(() => result.current.open({ mode: "new" }));

    expect(mockPush).toHaveBeenCalledTimes(1);
    expect(mockPush).toHaveBeenCalledWith("/admin/roles?filter=active&page=2&new=true", {
      scroll: false,
    });
  });

  it("pushes ?edit=<id> while preserving unrelated query params and removing new", () => {
    mockSearchParamsValue = "new=true&sort=name";
    const { result } = renderHook(() => useSheetSearchParam());

    act(() => result.current.open({ mode: "edit", id: "role-2" }));

    expect(mockPush).toHaveBeenCalledTimes(1);
    expect(mockPush).toHaveBeenCalledWith("/admin/roles?sort=name&edit=role-2", {
      scroll: false,
    });
  });

  it("replaces the URL without sheet params and preserves unrelated query params on close", () => {
    mockSearchParamsValue = "new=true&filter=active&edit=role-1";
    const { result } = renderHook(() => useSheetSearchParam());

    act(() => result.current.close());

    expect(mockReplace).toHaveBeenCalledTimes(1);
    expect(mockReplace).toHaveBeenCalledWith("/admin/roles?filter=active", { scroll: false });
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("can refresh after closing when requested", () => {
    mockPathname = "/admin/users";
    mockSearchParamsValue = "edit=user-1";
    const { result } = renderHook(() => useSheetSearchParam());

    act(() => result.current.close({ refresh: true }));

    expect(mockReplace).toHaveBeenCalledTimes(1);
    expect(mockReplace).toHaveBeenCalledWith("/admin/users", { scroll: false });
    expect(mockRefresh).toHaveBeenCalledTimes(1);
  });

  it("returns closed and warns when ?edit= has an empty value", () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    mockSearchParamsValue = "edit=";

    const { result } = renderHook(() => useSheetSearchParam());

    expect(result.current.state).toEqual({ mode: "closed" });
    expect(warnSpy).toHaveBeenCalledWith("[useSheetSearchParam] ignoring empty ?edit= value");
    warnSpy.mockRestore();
  });

  it("returns closed when ?new is not exactly 'true'", () => {
    mockSearchParamsValue = "new=false";

    const { result } = renderHook(() => useSheetSearchParam());

    expect(result.current.state).toEqual({ mode: "closed" });
  });

  it("throws when open() is called with an empty edit id", () => {
    mockSearchParamsValue = "";
    const { result } = renderHook(() => useSheetSearchParam());

    expect(() => act(() => result.current.open({ mode: "edit", id: "" }))).toThrowError(
      /requires a non-empty id/,
    );
    expect(mockPush).not.toHaveBeenCalled();
  });
});
