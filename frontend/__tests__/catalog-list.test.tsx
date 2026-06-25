// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
}));

// Stub CatalogClient — the RSC-level tests only need to verify what props the
// server component passes down. The client component has its own test file.
vi.mock("@/app/catalog/catalog-client", () => ({
  default: ({
    initialConnection,
  }: {
    initialConnection: {
      edges: { cursor: string; node: { id: string; name: string } }[];
      pageInfo: unknown;
      totalCount: number;
    } | null;
  }) => (
    <div data-testid="catalog-client">
      {initialConnection?.edges.map((e) => (
        <span key={e.node.id}>{e.node.name}</span>
      ))}
      {(!initialConnection || initialConnection.edges.length === 0) && (
        <span data-testid="empty-connection" />
      )}
    </div>
  ),
}));

// ---------------------------------------------------------------------------
// Imports — after vi.mock declarations
// ---------------------------------------------------------------------------

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import CatalogPage, { CatalogContent } from "@/app/catalog/page";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeConnection(items: { id: string; name: string; cardCount: number }[] = []): {
  masterCatalog: unknown;
} {
  return {
    masterCatalog: {
      __typename: "MasterCatalogConnection" as const,
      edges: items.map((item) => ({
        __typename: "MasterCatalogEdge" as const,
        cursor: item.id,
        node: {
          __typename: "MasterCardgroup" as const,
          description: null,
          ...item,
        },
      })),
      pageInfo: {
        __typename: "PageInfo" as const,
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: items[0]?.id ?? null,
        endCursor: items[items.length - 1]?.id ?? null,
      },
      totalCount: items.length,
    },
  };
}

function mockGqlFetch(data: unknown): void {
  vi.mocked(gqlFetch).mockResolvedValue(data as never);
}

function mockGqlFetchError(err: Error): void {
  vi.mocked(gqlFetch).mockRejectedValue(err);
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("CatalogPage", () => {
  it("renders both deck names when the user is logged in", async () => {
    // Auth gate lives in CatalogPage; data fetch lives in CatalogContent. Call
    // CatalogContent directly so the test is not blocked by the Suspense boundary
    // that CatalogPage returns.
    mockGqlFetch(
      makeConnection([
        { id: "master-001", name: "Business English", cardCount: 42 },
        { id: "master-002", name: "JLPT N3 Kanji", cardCount: 100 },
      ]),
    );

    const tree = await CatalogContent();
    render(tree as React.ReactElement);

    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(redirect).not.toHaveBeenCalled();
  });

  it("renders the empty connection when no published decks exist", async () => {
    mockGqlFetch(makeConnection([]));

    const tree = await CatalogContent();
    render(tree as React.ReactElement);

    expect(screen.getByTestId("empty-connection")).toBeInTheDocument();
    expect(redirect).not.toHaveBeenCalled();
  });

  it("redirects to /login when no user is signed in", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));

    await expect(CatalogPage()).rejects.toThrow("REDIRECT:/login");

    expect(redirect).toHaveBeenCalledWith("/login");
    // gqlFetch must not be reached when the auth gate already failed.
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("rethrows a non-auth gqlFetch error so the error boundary handles it", async () => {
    const networkErr = new Error("network failure");
    mockGqlFetchError(networkErr);

    await expect(CatalogContent()).rejects.toBe(networkErr);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("redirects to /login when gqlFetch raises an UNAUTHENTICATED GraphQL error", async () => {
    const authErr = new Error(
      `GraphQL errors: ${JSON.stringify([{ message: "no session", extensions: { code: "UNAUTHENTICATED" } }])}`,
    );
    mockGqlFetchError(authErr);

    await expect(CatalogContent()).rejects.toThrow("REDIRECT:/login");

    expect(redirect).toHaveBeenCalledWith("/login");
  });
});
