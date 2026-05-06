// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Mock server-redirect so tests can control whether the error is surfaced as a
// redirect or as a plain rethrow. The real implementation calls redirect() only
// for UNAUTHENTICATED errors; here the stub unconditionally rethrows, which is
// sufficient for the cases in this file that reach the server-redirect path.
vi.mock("@/lib/apollo/server-redirect", () => ({
  redirectIfUnauthenticated: vi.fn((err: unknown) => {
    throw err;
  }),
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
import CardgroupsPage from "@/app/cardgroups/page";
import { gqlFetch } from "@/lib/apollo/server";
import { makeCardgroup } from "./fixtures/cardgroups";
import { adminUserFixture, generalUserFixture } from "./fixtures/users";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

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
    setMockSupabaseUser({ id: adminUserFixture.id });

    const cg1 = makeCardgroup({ id: "cardgroup-001", name: "Spanish Vocab" });
    const cg2 = makeCardgroup({ id: "cardgroup-002", name: "Japanese Kanji" });

    mockGqlFetch(makeConnection([cg1, cg2]));

    const tree = await CardgroupsPage();
    render(tree as React.ReactElement);

    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Japanese Kanji")).toBeInTheDocument();

    expect(redirect).not.toHaveBeenCalled();
  });

  it("renders the empty-state hint when the user has no cardgroups", async () => {
    setMockSupabaseUser({ id: generalUserFixture.id });

    mockGqlFetch(makeConnection([]));

    const tree = await CardgroupsPage();
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
    setMockSupabaseUser({ id: generalUserFixture.id });

    const networkErr = new Error("network failure");
    mockGqlFetchError(networkErr);

    await expect(CardgroupsPage()).rejects.toBe(networkErr);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("rethrows when Supabase getUser() returns an error", async () => {
    setMockSupabaseUserError(new Error("supabase boom"));

    await expect(CardgroupsPage()).rejects.toThrow("supabase boom");

    expect(redirect).not.toHaveBeenCalled();
  });
});
