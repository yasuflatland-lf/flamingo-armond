// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { redirect } from "next/navigation";
import LearnIndexPage from "./page";
import { gqlFetch } from "@/lib/apollo/server";

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("LearnIndexPage", () => {
  test("anonymous user (user=null, no error) is redirected to /login", async () => {
    setMockSupabaseUser(null);
    await expect(LearnIndexPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("AuthSessionMissingError is silenced and user is redirected to /login", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);
    await expect(LearnIndexPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  test("non-AuthSessionMissingError from getUser() is logged with [learn-index] prefix and rethrown", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);
    await expect(LearnIndexPage()).rejects.toBe(transportError);
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[learn-index]"),
      transportError.name,
      transportError.message,
    );
  });

  test("UNAUTHENTICATED from gqlFetch redirects to /login", async () => {
    setMockSupabaseUser({ id: "u-1", email: "u@test" });
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );
    await expect(LearnIndexPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
  });

  test("non-UNAUTHENTICATED gqlFetch error is logged and rethrown", async () => {
    setMockSupabaseUser({ id: "u-1", email: "u@test" });
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);
    await expect(LearnIndexPage()).rejects.toBe(otherErr);
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[learn-index] me query failed"),
      otherErr,
    );
  });

  test("user with lastViewedCardgroup is redirected to /learn/<id>", async () => {
    setMockSupabaseUser({ id: "u-1", email: "u@test" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", lastViewedCardgroup: { id: "cg-42" } },
      myCardgroups: [{ id: "cg-42" }],
    } as never);
    await expect(LearnIndexPage()).rejects.toThrow(`${REDIRECT_PREFIX}/learn/cg-42`);
  });

  test("URL-encodes the destination id so reserved chars do not break the route", async () => {
    setMockSupabaseUser({ id: "u-1", email: "u@test" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", lastViewedCardgroup: { id: "a/b" } },
      myCardgroups: [],
    } as never);
    await expect(LearnIndexPage()).rejects.toThrow(`${REDIRECT_PREFIX}/learn/a%2Fb`);
  });

  test("user without lastViewedCardgroup is redirected to /cardgroups", async () => {
    setMockSupabaseUser({ id: "u-1", email: "u@test" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", lastViewedCardgroup: null },
      myCardgroups: [],
    } as never);
    await expect(LearnIndexPage()).rejects.toThrow(`${REDIRECT_PREFIX}/cardgroups`);
  });
});
