import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockCreateSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../__tests__/utils/mock-supabase";

// next/navigation mock — `redirect` throws so the server component aborts the
// same way Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { redirect } from "next/navigation";
import HomePage from "@/app/page";
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

describe("HomePage (root redirect)", () => {
  test("anonymous user (user=null, no error) is redirected to /login", async () => {
    setMockSupabaseUser(null);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("AuthSessionMissingError is silenced and user is redirected to /login", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("non-AuthSessionMissingError from getUser() is logged and rethrown", async () => {
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

  test("UNAUTHENTICATED from gqlFetch redirects to /login", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("non-UNAUTHENTICATED gqlFetch error is rethrown", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(HomePage()).rejects.toBe(otherErr);
    expect(redirect).not.toHaveBeenCalled();
  });

  test("user with lastViewedCardgroup is redirected to /learn/{id}", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: "Alice", lastViewedCardgroup: { id: "cg-42" } },
      myCardgroupsConnection: {
        __typename: "CardgroupConnection",
        totalCount: 1,
        edges: [],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: null,
          endCursor: null,
        },
      },
    } as never);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/learn/cg-42`);
    expect(redirect).toHaveBeenCalledWith("/learn/cg-42");
  });

  test("user with no lastViewedCardgroup but >=1 myCardgroupsConnection.totalCount → /cardgroups", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: "Alice", lastViewedCardgroup: null },
      myCardgroupsConnection: {
        __typename: "CardgroupConnection",
        totalCount: 1,
        edges: [],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: null,
          endCursor: null,
        },
      },
    } as never);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/cardgroups`);
    expect(redirect).toHaveBeenCalledWith("/cardgroups");
  });

  test("user with displayName: null (not onboarded) → /onboarding", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: null, lastViewedCardgroup: null },
      myCardgroupsConnection: {
        __typename: "CardgroupConnection",
        totalCount: 0,
        edges: [],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: null,
          endCursor: null,
        },
      },
    } as never);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/onboarding`);
    expect(redirect).toHaveBeenCalledWith("/onboarding");
  });

  test("throws when myCardgroupsConnection is null in the GraphQL response", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: "Alice", lastViewedCardgroup: null },
      myCardgroupsConnection: null,
    } as never);

    await expect(HomePage()).rejects.toThrow(
      /myCardgroupsConnection missing from root redirect data/,
    );
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[home]"));
  });

  test("onboarded user with no lastViewed and no cardgroups → /cardgroups/new?welcome=1", async () => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: "Alice", lastViewedCardgroup: null },
      myCardgroupsConnection: {
        __typename: "CardgroupConnection",
        totalCount: 0,
        edges: [],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: null,
          endCursor: null,
        },
      },
    } as never);

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/cardgroups/new?welcome=1`);
    expect(redirect).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
  });
});
