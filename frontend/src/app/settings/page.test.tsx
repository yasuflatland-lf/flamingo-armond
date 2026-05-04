// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

// next/navigation mock — `redirect` throws so the server component aborts the
// same way Next.js's server runtime does.
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
import SettingsPage from "@/app/settings/page";

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("SettingsPage (auth gate)", () => {
  test("anonymous user (user=null, no error) is redirected to /login", async () => {
    setMockSupabaseUser(null);

    await expect(SettingsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("AuthSessionMissingError is silenced and user is redirected to /login", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(SettingsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  test("non-AuthSessionMissingError from getUser() is logged and rethrown", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    await expect(SettingsPage()).rejects.toBe(transportError);
    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[settings]"),
      transportError.name,
      transportError.message,
    );
  });

  test("signed-in user renders h1 with Settings and Coming soon placeholder", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });

    const tree = await SettingsPage();
    render(tree as React.ReactElement);

    expect(screen.getByRole("heading", { level: 1, name: "Settings" })).toBeInTheDocument();
    expect(screen.getByText("Coming soon.")).toBeInTheDocument();
  });
});
