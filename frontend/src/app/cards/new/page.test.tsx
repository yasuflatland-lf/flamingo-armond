import { Suspense } from "react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

// next/navigation mock — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
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

// next-intl/server — the page resolves the Cards namespace via getTranslations.
vi.mock("next-intl/server", async () => {
  const enMessages = (await import("../../../../messages/en.json")).default;
  return {
    getTranslations: vi.fn(
      async (namespace: string) => (key: string) =>
        (enMessages as Record<string, Record<string, string>>)[namespace]?.[key] ?? key,
    ),
  };
});

// Stub CardsNewClient — it is a "use client" component that requires an
// ApolloProvider. The RSC page test only needs to verify props are forwarded
// correctly; the client component has its own dedicated test file.
vi.mock("./cards-new-client", () => ({
  default: ({
    initialCardgroupId,
    forcePickerOpen,
    myCardgroups,
  }: {
    initialCardgroupId: string | null;
    forcePickerOpen: boolean;
    myCardgroups: { id: string; name: string }[];
  }) => (
    <div
      data-testid="cards-new-client"
      data-initial-cardgroup-id={initialCardgroupId ?? ""}
      data-force-picker-open={String(forcePickerOpen)}
      data-cardgroups-count={myCardgroups.length}
    />
  ),
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { CardsNewSkeleton } from "@/app/cards/new/_components/cards-new-skeleton";
import CardsNewPage, { CardsNewContent } from "@/app/cards/new/page";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type Cardgroup = { id: string; name: string };

function makeBootstrapData(opts: { myCardgroups: Cardgroup[]; lastViewedId?: string | null }) {
  return {
    me: {
      id: "u-1",
      lastViewedCardgroup: opts.lastViewedId ? { id: opts.lastViewedId } : null,
    },
    myCardgroupsConnection: {
      __typename: "CardgroupConnection" as const,
      edges: opts.myCardgroups.map((cg) => ({
        __typename: "CardgroupEdge" as const,
        cursor: cg.id,
        node: cg,
      })),
      pageInfo: {
        __typename: "PageInfo" as const,
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: null,
        endCursor: null,
      },
      totalCount: opts.myCardgroups.length,
    },
  };
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Auth branches
// ---------------------------------------------------------------------------

describe("CardsNewPage — auth branches", () => {
  test("redirects when unauthenticated (anonymous) → redirect /login, gqlFetch not called", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "anonymous" }));

    await expect(CardsNewPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      `${REDIRECT_PREFIX}/login`,
    );

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("proceeds when authenticated → gqlFetch is not blocked by auth gate", async () => {
    // gqlFetch will reject, but only AFTER the auth gate passes — proving the
    // gate did not redirect.
    vi.mocked(gqlFetch).mockRejectedValueOnce(new Error("network"));

    await expect(CardsNewContent({ cardgroupParam: undefined })).rejects.toThrow("network");

    expect(redirect).not.toHaveBeenCalledWith("/login");
  });
});

// ---------------------------------------------------------------------------
// Suspense boundary
// ---------------------------------------------------------------------------

describe("CardsNewPage — Suspense boundary", () => {
  test("wraps the data-dependent subtree in <Suspense fallback={<CardsNewSkeleton />}>", async () => {
    // gqlFetch is unused here because we only inspect the synchronous JSX tree
    // returned by the page (we do not invoke CardsNewContent).
    const result = await CardsNewPage({ searchParams: Promise.resolve({}) });

    // Find the Suspense element in the tree.
    const findSuspense = (node: unknown): Record<string, unknown> | null => {
      if (node == null || typeof node !== "object") return null;
      const el = node as Record<string, unknown>;
      if ("type" in el && el.type === Suspense) return el;
      if ("props" in el && el.props != null) {
        const children = (el.props as Record<string, unknown>).children;
        if (Array.isArray(children)) {
          for (const child of children) {
            const found = findSuspense(child);
            if (found) return found;
          }
        } else if (children != null) {
          return findSuspense(children);
        }
      }
      return null;
    };

    const suspenseEl = findSuspense(result);
    expect(suspenseEl).not.toBeNull();
    const fallback = (suspenseEl?.props as { fallback?: { type?: unknown } }).fallback;
    expect(fallback).toBeDefined();
    expect((fallback as { type?: unknown }).type).toBe(CardsNewSkeleton);
  });
});

// ---------------------------------------------------------------------------
// gqlFetch error branches
// ---------------------------------------------------------------------------

describe("CardsNewPage — gqlFetch error branches", () => {
  test("UNAUTHENTICATED from gqlFetch → redirect /login", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(CardsNewContent({ cardgroupParam: undefined })).rejects.toThrow(
      `${REDIRECT_PREFIX}/login`,
    );

    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("non-UNAUTHENTICATED gqlFetch error is rethrown", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(CardsNewContent({ cardgroupParam: undefined })).rejects.toBe(otherErr);

    expect(redirect).not.toHaveBeenCalled();

    // PII-redacted payload: only `name` is logged, never `message`.
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[cards-new] gqlFetch failed:",
      expect.objectContaining({ name: expect.any(String) }),
    );
    // Assert that `message` (which may carry user-supplied content) is absent.
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  test("throws when myCardgroupsConnection is null in the bootstrap response", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", lastViewedCardgroup: null },
      myCardgroupsConnection: null,
    } as never);

    await expect(CardsNewContent({ cardgroupParam: undefined })).rejects.toThrow(
      /myCardgroupsConnection missing from bootstrap data/,
    );
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[cards-new]"));
  });
});

// ---------------------------------------------------------------------------
// Cardgroup resolution branches
// ---------------------------------------------------------------------------

describe("CardsNewPage — cardgroup resolution", () => {
  test("branch 1: ?cardgroup owned by user → initialCardgroupId=that id, forcePickerOpen=false", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeBootstrapData({
        myCardgroups: [
          { id: "cg-1", name: "Group A" },
          { id: "cg-2", name: "Group B" },
        ],
        lastViewedId: "cg-1",
      }) as never,
    );

    const jsx = await CardsNewContent({ cardgroupParam: "cg-2" });
    const props = (
      jsx as {
        props: {
          initialCardgroupId: string | null;
          forcePickerOpen: boolean;
          myCardgroups: Cardgroup[];
        };
      }
    ).props;

    expect(props.initialCardgroupId).toBe("cg-2");
    expect(props.forcePickerOpen).toBe(false);
    expect(props.myCardgroups).toHaveLength(2);
  });

  test("branch 1 ownership-fail: ?cardgroup not owned → falls through to branch 2 (lastViewed)", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeBootstrapData({
        myCardgroups: [{ id: "cg-1", name: "Group A" }],
        lastViewedId: "cg-1",
      }) as never,
    );

    // "evil-id" is not in myCardgroups → ownership check fails → branch 2 kicks in
    const jsx = await CardsNewContent({ cardgroupParam: "evil-id" });
    const props = (
      jsx as { props: { initialCardgroupId: string | null; forcePickerOpen: boolean } }
    ).props;

    expect(props.initialCardgroupId).toBe("cg-1");
    expect(props.forcePickerOpen).toBe(false);
  });

  test("branch 2: no ?cardgroup, lastViewedCardgroup in myCardgroups → use lastViewed id", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeBootstrapData({
        myCardgroups: [{ id: "cg-1", name: "Group A" }],
        lastViewedId: "cg-1",
      }) as never,
    );

    const jsx = await CardsNewContent({ cardgroupParam: undefined });
    const props = (
      jsx as { props: { initialCardgroupId: string | null; forcePickerOpen: boolean } }
    ).props;

    expect(props.initialCardgroupId).toBe("cg-1");
    expect(props.forcePickerOpen).toBe(false);
  });

  test("branch 2 lastViewed not owned: falls through to branch 3", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeBootstrapData({
        myCardgroups: [{ id: "cg-1", name: "Group A" }],
        lastViewedId: "cg-other",
      }) as never,
    );

    const jsx = await CardsNewContent({ cardgroupParam: undefined });
    const props = (
      jsx as { props: { initialCardgroupId: string | null; forcePickerOpen: boolean } }
    ).props;

    // lastViewed "cg-other" is not in myCardgroups → falls to branch 3
    expect(props.initialCardgroupId).toBeNull();
    expect(props.forcePickerOpen).toBe(true);
  });

  test("branch 3: no ?cardgroup, no lastViewed, myCardgroups non-empty → null + forcePickerOpen", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeBootstrapData({
        myCardgroups: [{ id: "cg-1", name: "Group A" }],
        lastViewedId: null,
      }) as never,
    );

    const jsx = await CardsNewContent({ cardgroupParam: undefined });
    const props = (
      jsx as {
        props: {
          initialCardgroupId: string | null;
          forcePickerOpen: boolean;
          myCardgroups: Cardgroup[];
        };
      }
    ).props;

    expect(props.initialCardgroupId).toBeNull();
    expect(props.forcePickerOpen).toBe(true);
    expect(props.myCardgroups).toHaveLength(1);
  });

  test("branch 4: no cardgroups at all → redirect /cardgroups/new?welcome=1", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeBootstrapData({
        myCardgroups: [],
        lastViewedId: null,
      }) as never,
    );

    await expect(CardsNewContent({ cardgroupParam: undefined })).rejects.toThrow(
      `${REDIRECT_PREFIX}/cardgroups/new?welcome=1`,
    );

    expect(redirect).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
  });
});
