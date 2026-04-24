import { afterEach, describe, expect, it, vi } from "vitest";
import { createSupabaseServerClient } from "./server";

// Shared spy captured by vi.hoisted so mock factories can reference it
const mockCookieStore = vi.hoisted(() => ({
  getAll: vi.fn().mockReturnValue([]),
  set: vi.fn(),
}));

vi.mock("next/headers", () => ({
  cookies: vi.fn().mockResolvedValue(mockCookieStore),
}));

// Capture the options passed to createServerClient so tests can invoke callbacks
let capturedOptions: {
  cookies: {
    getAll: () => unknown;
    setAll: (c: { name: string; value: string; options: object }[]) => void;
  };
} | null = null;

vi.mock("@supabase/ssr", () => ({
  createServerClient: vi
    .fn()
    .mockImplementation((_url: string, _key: string, opts: typeof capturedOptions) => {
      capturedOptions = opts;
      return {};
    }),
}));

describe("createSupabaseServerClient", () => {
  afterEach(() => {
    vi.clearAllMocks();
    // Reset any one-off implementations so subsequent tests start clean
    mockCookieStore.set.mockReset();
    capturedOptions = null;
  });

  it("does NOT propagate exceptions thrown by cookieStore.set (silent-swallow contract)", async () => {
    mockCookieStore.set.mockImplementation(() => {
      throw new Error("read-only in Server Component");
    });

    await createSupabaseServerClient();

    expect(capturedOptions).not.toBeNull();
    // Calling setAll must not throw even though cookieStore.set throws
    expect(() =>
      capturedOptions?.cookies.setAll([{ name: "sb-token", value: "x", options: {} }]),
    ).not.toThrow();
  });

  it("invokes cookieStore.set for each entry when set does NOT throw", async () => {
    await createSupabaseServerClient();

    expect(capturedOptions).not.toBeNull();
    const entries = [
      { name: "sb-access-token", value: "abc", options: { path: "/" } },
      { name: "sb-refresh-token", value: "def", options: { path: "/" } },
    ];
    capturedOptions?.cookies.setAll(entries);

    expect(mockCookieStore.set).toHaveBeenCalledTimes(2);
    expect(mockCookieStore.set).toHaveBeenNthCalledWith(1, "sb-access-token", "abc", {
      path: "/",
    });
    expect(mockCookieStore.set).toHaveBeenNthCalledWith(2, "sb-refresh-token", "def", {
      path: "/",
    });
  });

  it("delegates getAll to cookieStore.getAll", async () => {
    mockCookieStore.getAll.mockReturnValueOnce([{ name: "session", value: "s1" }]);
    await createSupabaseServerClient();

    const result = capturedOptions?.cookies.getAll();
    expect(result).toEqual([{ name: "session", value: "s1" }]);
    expect(mockCookieStore.getAll).toHaveBeenCalledTimes(1);
  });
});
