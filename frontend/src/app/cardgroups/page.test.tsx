// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Mock next/navigation before importing the page
vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

// Mock createSupabaseServerClient
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

// Mock gqlFetch
vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Stub CardgroupsClient — it is a "use client" component that requires an
// ApolloProvider. The RSC page test only needs to verify props are forwarded
// correctly; the client component has its own dedicated test file.
vi.mock("./cardgroups-client", () => ({
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

import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import CardgroupsPage from "./page";

function makeSupabaseMock(user: { id: string } | null) {
  return {
    auth: {
      getUser: vi.fn().mockResolvedValue({
        data: { user },
        error: null,
      }),
    },
  };
}

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

describe("CardgroupsPage", () => {
  beforeEach(() => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
  });

  it("redirects to /login when no user is authenticated", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");
  });

  it("renders CardgroupsClient with empty connection when myCardgroupsConnection is empty", async () => {
    vi.mocked(gqlFetch).mockResolvedValue(makeConnection([]) as never);

    const jsx = await CardgroupsPage();
    render(jsx);

    expect(screen.getByTestId("cardgroups-client")).toBeInTheDocument();
    expect(screen.getByTestId("empty-connection")).toBeInTheDocument();
  });

  it("passes connection edges to CardgroupsClient", async () => {
    vi.mocked(gqlFetch).mockResolvedValue(
      makeConnection([
        { id: "cg-1", name: "Spanish Vocab", updatedAt: "2024-06-15T10:00:00.000Z" },
        { id: "cg-2", name: "Math Formulas", updatedAt: "2024-05-20T08:00:00.000Z" },
      ]) as never,
    );

    const jsx = await CardgroupsPage();
    render(jsx);

    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();
  });

  it("renders CardgroupsClient even with a single cardgroup", async () => {
    vi.mocked(gqlFetch).mockResolvedValue(
      makeConnection([
        { id: "cg-1", name: "Spanish Vocab", updatedAt: "2024-06-15T10:00:00.000Z" },
      ]) as never,
    );

    const jsx = await CardgroupsPage();
    render(jsx);

    expect(screen.getByTestId("cardgroups-client")).toBeInTheDocument();
    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
  });

  it("redirects to /login when MyCardgroupsConnectionQuery returns UNAUTHENTICATED", async () => {
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(
        'GraphQL errors: [{"message":"Unauthenticated","extensions":{"code":"UNAUTHENTICATED"}}]',
      ),
    );

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");
  });

  it("rethrows non-auth errors so the error boundary handles them", async () => {
    vi.mocked(gqlFetch).mockRejectedValue(new Error("Network unreachable"));

    await expect(CardgroupsPage()).rejects.toThrow("Network unreachable");
  });

  it("redirects to /login when gqlFetch throws an UNAUTHENTICATED error (structural parse)", async () => {
    // isUnauthenticatedGraphQLError parses extensions.code structurally — no
    // substring matching. This test verifies the structural redirect path works.
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(
        'GraphQL errors: [{"message":"Unauthenticated","extensions":{"code":"UNAUTHENTICATED"}}]',
      ),
    );

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");
  });
});
