// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  mockCreateSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

// Mock createSupabaseServerClient using the shared utility so per-test state is
// driven via setMockSupabaseUser / setMockSupabaseUserError.
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));

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
import { CardgroupsSkeleton } from "./_components/cardgroups-skeleton";
import CardgroupsPage, { CardgroupsContent } from "./page";

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

describe("CardgroupsPage — AuthSessionMissingError filter", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    resetMockSupabase();
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it("redirects to /login on AuthSessionMissingError without calling console.error or gqlFetch", async () => {
    const authMissing = new Error("Auth session missing!");
    authMissing.name = "AuthSessionMissingError";
    setMockSupabaseUserError(authMissing);

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");

    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(vi.mocked(gqlFetch)).not.toHaveBeenCalled();
  });

  it("calls console.error and rethrows when getUser returns a non-ignorable error", async () => {
    // Any error other than AuthSessionMissingError / stale-session is a real auth
    // failure and must bubble up so the error boundary handles it.
    const fakeError = new Error("something broke");
    fakeError.name = "NetworkAuthError";
    setMockSupabaseUserError(fakeError);

    await expect(CardgroupsPage()).rejects.toMatchObject({
      name: fakeError.name,
      message: fakeError.message,
    });

    // PII-redacted payload: only `name` is logged inside an object, never `message`.
    expect(consoleErrorSpy).toHaveBeenCalledWith("[cardgroups] getUser() failed:", {
      name: fakeError.name,
    });
    // Assert that `message` (which may carry user-supplied content) is absent.
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("redirects to /login on stale-session (deleted user) auth error", async () => {
    // isStaleSessionError: name === "AuthApiError" AND message includes "does not exist".
    // isIgnorableAuthError returns true for stale-session, so it does NOT throw;
    // the subsequent `isStaleSessionError(authErr)` check redirects to /login.
    const staleErr = new Error("User from sub claim does not exist");
    staleErr.name = "AuthApiError";
    setMockSupabaseUserError(staleErr);

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");

    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(vi.mocked(gqlFetch)).not.toHaveBeenCalled();
  });
});

describe("CardgroupsPage — outer auth + Suspense shell", () => {
  beforeEach(() => {
    resetMockSupabase();
    setMockSupabaseUser({ id: "user-1" });
  });

  it("redirects to /login when no user is authenticated", async () => {
    setMockSupabaseUser(null);

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");
  });

  it("returns a <Suspense> boundary with <CardgroupsSkeleton /> as fallback", async () => {
    // The route shell must stream: outer page returns a Suspense element whose
    // fallback is the cardgroups skeleton, so navigations show the skeleton
    // immediately while the GraphQL fetch resolves inside CardgroupsContent.
    const jsx = await CardgroupsPage();

    expect(jsx.type).toBe(Suspense);
    expect(jsx.props.fallback.type).toBe(CardgroupsSkeleton);
    expect(jsx.props.children.type).toBe(CardgroupsContent);
  });
});

// ---------------------------------------------------------------------------
// CardgroupsContent — inner async server component that performs the gqlFetch.
// Tested directly (not through Suspense) because vitest's renderer cannot
// resolve the suspended subtree synchronously and React Server Components
// expose this seam by design.
// ---------------------------------------------------------------------------

describe("CardgroupsContent", () => {
  it("renders CardgroupsClient with empty connection when myCardgroupsConnection is empty", async () => {
    vi.mocked(gqlFetch).mockResolvedValue(makeConnection([]) as never);

    const jsx = await CardgroupsContent();
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

    const jsx = await CardgroupsContent();
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

    const jsx = await CardgroupsContent();
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

    await expect(CardgroupsContent()).rejects.toThrow("REDIRECT:/login");
  });

  it("rethrows non-auth errors so the error boundary handles them", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      vi.mocked(gqlFetch).mockRejectedValue(new Error("Network unreachable"));

      await expect(CardgroupsContent()).rejects.toThrow("Network unreachable");

      // PII-redacted payload: only `name` is logged, never `message`.
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "[cardgroups] gqlFetch failed:",
        expect.objectContaining({ name: expect.any(String) }),
      );
      // Assert that `message` (which may carry user-supplied content) is absent.
      expect(consoleErrorSpy).not.toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({ message: expect.anything() }),
      );
    } finally {
      consoleErrorSpy.mockRestore();
    }
  });
});
