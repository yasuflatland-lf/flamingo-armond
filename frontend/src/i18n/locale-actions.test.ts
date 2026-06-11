import { afterEach, describe, expect, it, vi } from "vitest";
import type { Locale } from "./config";
import { setUserLocale } from "./locale-actions";

// Shared cookie-store spy captured by vi.hoisted so the mock factory can
// reference it (mirrors the pattern in lib/supabase/server.test.ts).
const mockCookieStore = vi.hoisted(() => ({
  set: vi.fn(),
}));

vi.mock("next/headers", () => ({
  cookies: vi.fn().mockResolvedValue(mockCookieStore),
}));

afterEach(() => {
  mockCookieStore.set.mockReset();
  vi.restoreAllMocks();
});

describe("setUserLocale", () => {
  it("writes the NEXT_LOCALE cookie with the load-bearing attributes", async () => {
    await setUserLocale("ja");

    expect(mockCookieStore.set).toHaveBeenCalledTimes(1);
    expect(mockCookieStore.set).toHaveBeenCalledWith(
      "NEXT_LOCALE",
      "ja",
      // The cookie name, the one-year maxAge, the path, and SameSite=Lax are the
      // integration contract with next-intl's request config + the persistence
      // promise; pin them so a regression (e.g. maxAge dropping to one day, or
      // SameSite drifting to "none") is caught.
      expect.objectContaining({
        path: "/",
        maxAge: 60 * 60 * 24 * 365,
        sameSite: "lax",
      }),
    );
  });

  it("persists English as well", async () => {
    await setUserLocale("en");

    expect(mockCookieStore.set).toHaveBeenCalledWith(
      "NEXT_LOCALE",
      "en",
      expect.objectContaining({ path: "/", sameSite: "lax" }),
    );
  });

  it("ignores an unsupported locale value (a network caller can bypass the compile-time type)", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    await setUserLocale("fr" as Locale);

    expect(mockCookieStore.set).not.toHaveBeenCalled();
    expect(warn).toHaveBeenCalled();
  });

  it("rethrows when the cookie write fails so the caller can surface it", async () => {
    const boom = new Error("cookie store unavailable");
    mockCookieStore.set.mockImplementation(() => {
      throw boom;
    });
    vi.spyOn(console, "error").mockImplementation(() => {});

    await expect(setUserLocale("ja")).rejects.toThrow(boom);
  });
});
