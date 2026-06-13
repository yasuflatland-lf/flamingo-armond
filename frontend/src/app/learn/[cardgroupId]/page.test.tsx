// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

vi.mock("./learn-client", () => ({
  LearnClient: ({
    cardgroupId,
    initialCards,
    displayMode,
  }: {
    cardgroupId: string;
    initialCards: unknown[];
    displayMode: string;
  }) => (
    <div data-testid="learn-client">
      {cardgroupId}:{initialCards.length}:{displayMode}
    </div>
  ),
}));

import { headers } from "next/headers";
import { gqlFetch } from "@/lib/apollo/server";
import { LearnSkeleton } from "./_components/learn-skeleton";
import LearnPage from "./page";

// RTL cannot render async RSCs inside <Suspense>, so we walk the JSX tree
// directly to introspect the page structure without triggering data fetches.
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

// Extracts the <Suspense> child and fallback from the page JSX tree.
// Throws if no boundary is found — that would indicate a streaming regression.
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
// LearnContent — non-UNAUTHENTICATED gqlFetch error rethrow with PII redaction
// ---------------------------------------------------------------------------

describe("LearnPage — LearnContent gqlFetch error branches", () => {
  beforeEach(() => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
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
        "[learn] gqlFetch failed:",
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
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
  });

  afterEach(() => {
    vi.mocked(gqlFetch).mockReset();
  });

  it("redirects to /login when unauthenticated", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "anonymous" }));

    await expect(LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
  });

  it("redirects to /login when auth status is stale", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "stale" }));

    await expect(LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
  });

  it("redirects to /login when auth status is error", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "error" }));

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

  it("redirects to /login when LearnNextDueCards returns UNAUTHENTICATED", async () => {
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);

    await expect(childType(childProps)).rejects.toThrow("REDIRECT:/login");
  });

  it("redirects to /cardgroups when LearnNextDueCards returns BAD_USER_INPUT (missing cardgroup)", async () => {
    // The usecase authorizes the cardgroup inside the cards query, so a missing
    // cardgroup surfaces as BAD_USER_INPUT — the surviving form of the old
    // CardgroupQuery existence guard.
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(
        `GraphQL errors: ${JSON.stringify([
          { extensions: { code: "BAD_USER_INPUT", field: "cardgroupId" } },
        ])}`,
      ),
    );

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);

    await expect(childType(childProps)).rejects.toThrow("REDIRECT:/cardgroups");
  });

  it("server-renders the first card batch into LearnClient", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
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
      me: null,
    } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);
    const inner = await childType(childProps);
    render(inner as React.ReactElement);

    // Format: cardgroupId:initialCards.length:displayMode
    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:1:FLIP_TO_REVEAL");
  });

  it("server-renders an empty due batch into LearnClient", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      learnNextDueCards: [],
      me: null,
    } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);
    const inner = await childType(childProps);
    render(inner as React.ReactElement);

    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:0:FLIP_TO_REVEAL");
  });

  it("passes the ride-along learn display mode into LearnClient", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      learnNextDueCards: [],
      me: {
        learnDisplayMode: "ALWAYS_VISIBLE",
      },
    } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    const { childType, childProps } = getSuspenseChild(jsx);
    const inner = await childType(childProps);
    render(inner as React.ReactElement);

    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:0:ALWAYS_VISIBLE");
  });
});
