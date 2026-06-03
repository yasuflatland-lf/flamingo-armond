import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

// next/navigation mock — `redirect` throws so the server component aborts the
// same way Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import HomePage from "@/app/page";
import { gqlFetch } from "@/lib/apollo/server";

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  // Default: authenticated
  vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("HomePage (root redirect)", () => {
  test("redirects to /login when unauthenticated (x-auth-status: anonymous)", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "anonymous" }));

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("redirects to /login when stale session (x-auth-status: stale)", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "stale" }));

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("redirects to /login when auth error (x-auth-status: error)", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "error" }));

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("proceeds when authenticated (x-auth-status: authenticated)", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
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
    expect(gqlFetch).toHaveBeenCalled();
  });

  test("UNAUTHENTICATED from gqlFetch redirects to /login", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(HomePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("non-UNAUTHENTICATED gqlFetch error is rethrown", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(HomePage()).rejects.toBe(otherErr);
    expect(redirect).not.toHaveBeenCalled();
  });

  test("user with lastViewedCardgroup is redirected to /learn/{id}", async () => {
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
