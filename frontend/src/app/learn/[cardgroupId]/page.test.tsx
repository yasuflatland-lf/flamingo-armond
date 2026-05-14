// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../../__tests__/utils/mock-supabase";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

// Mock createSupabaseServerClient using the shared utility so per-test state is
// driven via setMockSupabaseUser / setMockSupabaseUserError.
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

vi.mock("./learn-client", () => ({
  LearnClient: ({
    cardgroupId,
    initialCards,
    lastViewedCardgroupId,
  }: {
    cardgroupId: string;
    initialCards: unknown[];
    lastViewedCardgroupId: string | null;
  }) => (
    <div data-testid="learn-client">
      {cardgroupId}:{initialCards.length}:{lastViewedCardgroupId ?? "null"}
    </div>
  ),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { LearnSkeleton } from "./_components/learn-skeleton";
import LearnPage from "./page";

/**
 * Recursively search a React element tree for a node whose type matches
 * `predicate`. Returns the matching element, or null. Used to introspect the
 * page output without depending on RTL's ability to render async server
 * components inside `<Suspense>`.
 */
type ReactElementLike = {
  type: unknown;
  props: Record<string, unknown>;
};

function findElement(
  node: unknown,
  predicate: (el: ReactElementLike) => boolean,
): ReactElementLike | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  if ("type" in el && "props" in el) {
    const candidate = el as unknown as ReactElementLike;
    if (predicate(candidate)) return candidate;
    const children = (candidate.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElement(child, predicate);
        if (found) return found;
      }
    } else if (children != null) {
      return findElement(children, predicate);
    }
  }
  return null;
}

/**
 * Locate the `<Suspense>` element inside the page output and return both its
 * child (the async `<LearnContent />` element) and its fallback. Throws if no
 * Suspense boundary is found — that case indicates a regression of the
 * streaming structure the route is meant to provide.
 */
function getSuspenseChild(jsx: unknown): {
  childType: (props: { cardgroupId: string }) => Promise<React.ReactNode>;
  childProps: { cardgroupId: string };
  fallback: unknown;
} {
  const suspense = findElement(jsx, (el) => el.type === Suspense);
  if (!suspense) throw new Error("No <Suspense> boundary found in page output");
  const child = suspense.props.children as ReactElementLike;
  return {
    childType: child.type as (props: { cardgroupId: string }) => Promise<React.ReactNode>,
    childProps: child.props as { cardgroupId: string },
    fallback: suspense.props.fallback,
  };
}

// ---------------------------------------------------------------------------
// AuthSessionMissingError filter — .claude/rules/frontend-rsc-error-handling.md
// § "AuthSessionMissingError is the no session signal"
// ---------------------------------------------------------------------------

describe("LearnPage — AuthSessionMissingError filter", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    resetMockSupabase();
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it("redirects to /login on AuthSessionMissingError without calling console.error", async () => {
    // AuthSessionMissingError is the normal anonymous-visitor signal; it must NOT
    // be treated as a real failure (no console.error, just redirect).
    const authMissing = new Error("Auth session missing!");
    authMissing.name = "AuthSessionMissingError";
    setMockSupabaseUserError(authMissing);

    await expect(
      LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) }),
    ).rejects.toThrow("REDIRECT:/login");

    // The filter must NOT log a console.error for the expected anonymous path.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  it("calls console.error and rethrows when getUser returns a non-AuthSessionMissingError", async () => {
    // Any error other than AuthSessionMissingError is a real auth failure and
    // must bubble up so the error boundary handles it.
    const fakeError = new Error("something broke");
    fakeError.name = "AuthApiError";
    setMockSupabaseUserError(fakeError);

    await expect(
      LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) }),
    ).rejects.toMatchObject({
      name: fakeError.name,
      message: fakeError.message,
    });

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[learn] getUser() failed:",
      fakeError.name,
      fakeError.message,
    );
  });
});

// ---------------------------------------------------------------------------
// LearnContent — non-UNAUTHENTICATED gqlFetch error rethrow with PII redaction
// ---------------------------------------------------------------------------

describe("LearnPage — LearnContent gqlFetch error branches", () => {
  beforeEach(() => {
    resetMockSupabase();
    setMockSupabaseUser({ id: "user-1" });
  });

  it("rethrows non-auth gqlFetch errors and logs PII-redacted payload", async () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      vi.mocked(gqlFetch).mockRejectedValue(new Error("Network unreachable"));

      const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
      const { childType, childProps } = getSuspenseChild(jsx);

      await expect(childType(childProps)).rejects.toThrow("Network unreachable");

      // PII-redacted payload: only `name` is logged, never `message`.
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "[learn] gqlFetch batch failed:",
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

describe("LearnPage", () => {
  beforeEach(() => {
    resetMockSupabase();
    setMockSupabaseUser({ id: "user-1" });
  });

  it("redirects to /login when no user is authenticated", async () => {
    setMockSupabaseUser(null);

    await expect(LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
  });

  it("wraps the data-dependent subtree in <Suspense> with a <LearnSkeleton /> fallback", async () => {
    // gqlFetch must not be called by the outer page — it runs only inside the
    // Suspense child, which the test does not invoke here.
    vi.mocked(gqlFetch).mockReset();

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { fallback } = getSuspenseChild(jsx);

    expect(fallback).toBeTruthy();
    expect((fallback as ReactElementLike).type).toBe(LearnSkeleton);
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("redirects to /login when the GraphQL batch returns UNAUTHENTICATED", async () => {
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);

    await expect(childType(childProps)).rejects.toThrow("REDIRECT:/login");
  });

  it("redirects to /cardgroups when the cardgroup is not found", async () => {
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ cardgroup: null } as never)
      .mockResolvedValueOnce({ learnNextDueCards: [] } as never)
      .mockResolvedValueOnce({
        me: { id: "user-1", lastViewedCardgroup: null },
        myCardgroups: [],
      } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);

    await expect(childType(childProps)).rejects.toThrow("REDIRECT:/cardgroups");
  });

  it("server-renders the first card batch into LearnClient", async () => {
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Spanish", updatedAt: "2026-04-30T00:00:00Z" },
      } as never)
      .mockResolvedValueOnce({
        learnNextDueCards: [
          {
            id: "c-1",
            front: "Hello",
            back: "Hola",
            userCardState: {
              due: "2026-04-30T00:00:00Z",
              state: 0,
            },
            cardgroupId: "cg-1",
          },
        ],
      } as never)
      .mockResolvedValueOnce({
        me: { id: "user-1", lastViewedCardgroup: { id: "cg-old" } },
        myCardgroups: [],
      } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);
    const inner = await childType(childProps);
    render(inner as React.ReactElement);

    // Format: cardgroupId:initialCards.length:lastViewedCardgroupId
    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:1:cg-old");
  });

  it("server-renders an empty due batch into LearnClient", async () => {
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Spanish", updatedAt: "2026-04-30T00:00:00Z" },
      } as never)
      .mockResolvedValueOnce({
        learnNextDueCards: [],
      } as never)
      .mockResolvedValueOnce({
        me: { id: "user-1", lastViewedCardgroup: null },
        myCardgroups: [],
      } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);
    const inner = await childType(childProps);
    render(inner as React.ReactElement);

    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:0:null");
  });
});
