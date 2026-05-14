import { Suspense } from "react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../../__tests__/utils/mock-supabase";

// next/navigation mock — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { redirect } from "next/navigation";
import { CardsNewSkeleton } from "@/app/cards/new/_components/cards-new-skeleton";
import CardsNewPage from "@/app/cards/new/page";
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
    myCardgroups: opts.myCardgroups,
  };
}

type CardsNewClientProps = {
  initialCardgroupId: string | null;
  forcePickerOpen: boolean;
  myCardgroups: Cardgroup[];
};

/**
 * Recursively search a React element tree for a node whose `type` display name
 * matches `componentName` and return its props.
 */
function findElementProps(node: unknown, componentName: string): CardsNewClientProps | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  // React element: { type, props, ... }
  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string; displayName?: string }).name === componentName
  ) {
    return el.props as CardsNewClientProps;
  }
  // Recurse into props.children
  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElementProps(child, componentName);
        if (found) return found;
      }
    } else if (children != null) {
      return findElementProps(children, componentName);
    }
  }
  return null;
}

/**
 * Locate a React element by its component function name. Used to descend into
 * the async `CardsNewContent` server component, whose children are not
 * reachable via `findElementProps` alone (the tree carries the function
 * reference, not its evaluated output).
 */
function findElement(
  node: unknown,
  componentName: string,
): { type: (props: unknown) => unknown; props: Record<string, unknown> } | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string }).name === componentName
  ) {
    return el as { type: (props: unknown) => unknown; props: Record<string, unknown> };
  }
  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElement(child, componentName);
        if (found) return found;
      }
    } else if (children != null) {
      return findElement(children, componentName);
    }
  }
  return null;
}

/**
 * Call the page component with the given searchParams, descend into the
 * async `CardsNewContent` server component, then traverse the resulting tree
 * to find the `<CardsNewClient>` element and return its props.
 */
async function renderPage(searchParams: Record<string, string> = {}) {
  const result = await CardsNewPage({ searchParams: Promise.resolve(searchParams) });
  const contentEl = findElement(result, "CardsNewContent");
  if (!contentEl) {
    throw new Error(
      "CardsNewContent element not found in the page output — page may have redirected",
    );
  }
  // CardsNewContent is an async server component — invoke it with its props
  // to obtain the resolved subtree that contains <CardsNewClient>.
  const contentResult = await (contentEl.type as (props: unknown) => Promise<unknown>)(
    contentEl.props,
  );
  const props = findElementProps(contentResult, "CardsNewClient");
  if (!props) {
    throw new Error(
      "CardsNewClient element not found in the CardsNewContent output — content may have redirected",
    );
  }
  return props;
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Auth branches
// ---------------------------------------------------------------------------

describe("CardsNewPage — auth branches", () => {
  test("anonymous user (user=null, no error) → redirect /login, gqlFetch not called", async () => {
    setMockSupabaseUser(null);

    await expect(CardsNewPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      `${REDIRECT_PREFIX}/login`,
    );

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("AuthSessionMissingError is silenced → redirect /login, no console.error", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(CardsNewPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      `${REDIRECT_PREFIX}/login`,
    );

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("non-ignorable auth error → console.error (PII-redacted) + rethrow", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    await expect(CardsNewPage({ searchParams: Promise.resolve({}) })).rejects.toBe(transportError);

    expect(redirect).not.toHaveBeenCalled();
    // PII-redacted payload: only `name` is logged inside an object, never `message`.
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[cards-new] getUser() failed:",
      { name: transportError.name },
    );
    // Assert that `message` (which may carry user-supplied content) is absent.
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  test("stale-session (deleted user) auth error → redirect /login, no console.error", async () => {
    // isStaleSessionError: name === "AuthApiError" AND message includes "does not exist".
    // isIgnorableAuthError returns true for stale-session, so it does NOT throw;
    // the subsequent `isStaleSessionError(authErr)` check redirects to /login.
    const staleErr = new Error("User from sub claim does not exist");
    staleErr.name = "AuthApiError";
    setMockSupabaseUserError(staleErr);

    await expect(CardsNewPage({ searchParams: Promise.resolve({}) })).rejects.toThrow(
      `${REDIRECT_PREFIX}/login`,
    );

    expect(redirect).toHaveBeenCalledWith("/login");
    // Stale-session is ignorable — must not log an error.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Suspense boundary
// ---------------------------------------------------------------------------

describe("CardsNewPage — Suspense boundary", () => {
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
  });

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
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
  });

  test("UNAUTHENTICATED from gqlFetch → redirect /login", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(renderPage({})).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("non-UNAUTHENTICATED gqlFetch error is rethrown", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(renderPage({})).rejects.toBe(otherErr);

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
});

// ---------------------------------------------------------------------------
// Cardgroup resolution branches
// ---------------------------------------------------------------------------

describe("CardsNewPage — cardgroup resolution", () => {
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
  });

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

    const props = await renderPage({ cardgroup: "cg-2" });

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
    const props = await renderPage({ cardgroup: "evil-id" });

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

    const props = await renderPage({});

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

    const props = await renderPage({});

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

    const props = await renderPage({});

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

    await expect(renderPage({})).rejects.toThrow(`${REDIRECT_PREFIX}/cardgroups/new?welcome=1`);

    expect(redirect).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
  });
});
