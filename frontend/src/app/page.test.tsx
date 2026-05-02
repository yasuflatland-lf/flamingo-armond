import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../__tests__/utils/mock-supabase";

// ---------------------------------------------------------------------------
// next/navigation mock — `redirect` throws so the server component aborts the
// same way Next.js's server runtime does.
// ---------------------------------------------------------------------------

const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

import { redirect } from "next/navigation";
import HomePage from "@/app/page";

// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------

describe("HomePage (root redirect)", () => {
  test("anonymous user is redirected to /login", async () => {
    setMockSupabaseUser(null);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("logged-in user is redirected to /cardgroups", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/cardgroups`);
    expect(redirect).toHaveBeenCalledWith("/cardgroups");
  });

  test("non-AuthSessionMissingError from getUser() is rethrown", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    await expect(HomePage()).rejects.toBe(transportError);
    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[home]"),
      transportError.name,
      transportError.message,
    );
  });

  test("AuthSessionMissingError is silenced and user is redirected to /login", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });
});
