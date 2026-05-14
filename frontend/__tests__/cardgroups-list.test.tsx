// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
// The mock-supabase import must precede the `vi.mock("@/lib/supabase/server", ...)`
// factory below: the factory references `mockCreateSupabaseServerClient`, and
// Vitest's hoisting of `vi.mock` produces a TDZ error if the binding is
// imported later in source order than the factory that references it.
import {
  mockCreateSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// Stub CardgroupsClient — the RSC-level tests only need to verify what props
// the server component passes down. The client component has its own test file.
vi.mock("@/app/cardgroups/cardgroups-client", () => ({
  default: ({
    initialConnection,
  }: {
    initialConnection: {
      edges: { cursor: string; node: { id: string; name: string } }[];
      pageInfo: unknown;
      totalCount: number;
    } | null;
  }) => (
    <div data-testid="cardgroups-client">
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

import { redirect } from "next/navigation";
import CardgroupsPage, { CardgroupsContent } from "@/app/cardgroups/page";
import { gqlFetch } from "@/lib/apollo/server";
import { makeCardgroup } from "./fixtures/cardgroups";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeConnection(items: { id: string; name: string; updatedAt: string }[] = []) {
  return {
    myCardgroupsConnection: {
      __typename: "CardgroupConnection" as const,
      edges: items.map((item) => ({
        __typename: "CardgroupEdge" as const,
        cursor: item.id,
        node: { __typename: "Cardgroup" as const, ...item },
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
  resetMockSupabase();
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("CardgroupsPage", () => {
  it("renders both cardgroup names and their links when the user is logged in", async () => {
    // Auth gate lives in CardgroupsPage; data fetch lives in CardgroupsContent.
    // Call CardgroupsContent directly so the test is not blocked by the Suspense
    // boundary that CardgroupsPage now returns.
    const cg1 = makeCardgroup({ id: "cardgroup-001", name: "Spanish Vocab" });
    const cg2 = makeCardgroup({ id: "cardgroup-002", name: "Japanese Kanji" });

    mockGqlFetch(makeConnection([cg1, cg2]));

    const tree = await CardgroupsContent();
    render(tree as React.ReactElement);

    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Japanese Kanji")).toBeInTheDocument();

    expect(redirect).not.toHaveBeenCalled();
  });

  it("renders the empty-state hint when the user has no cardgroups", async () => {
    // Auth gate lives in CardgroupsPage; data fetch lives in CardgroupsContent.
    // Call CardgroupsContent directly so the test is not blocked by the Suspense
    // boundary that CardgroupsPage now returns.
    mockGqlFetch(makeConnection([]));

    const tree = await CardgroupsContent();
    render(tree as React.ReactElement);

    expect(screen.getByTestId("empty-connection")).toBeInTheDocument();

    expect(redirect).not.toHaveBeenCalled();
  });

  it("redirects to /login when no user is signed in", async () => {
    // state.user remains null after resetMockSupabase — logged-out request
    setMockSupabaseUser(null);

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");

    expect(redirect).toHaveBeenCalledWith("/login");
    // gqlFetch must not be reached when the auth gate already failed.
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("rethrows a non-auth gqlFetch error so the error boundary handles it", async () => {
    // gqlFetch is called inside CardgroupsContent (not in the outer CardgroupsPage
    // shell which only runs the auth check and returns a Suspense boundary).
    const networkErr = new Error("network failure");
    mockGqlFetchError(networkErr);

    await expect(CardgroupsContent()).rejects.toBe(networkErr);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("rethrows when Supabase getUser() returns an error", async () => {
    setMockSupabaseUserError(new Error("supabase boom"));

    await expect(CardgroupsPage()).rejects.toThrow("supabase boom");

    expect(redirect).not.toHaveBeenCalled();
  });
});
