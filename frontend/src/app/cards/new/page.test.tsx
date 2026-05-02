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
import { gqlFetch } from "@/lib/apollo/server";
import CardsNewPage from "@/app/cards/new/page";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type Cardgroup = { id: string; name: string };

function makeBootstrapData(opts: {
  myCardgroups: Cardgroup[];
  lastViewedId?: string | null;
}) {
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
function findElementProps(
  node: unknown,
  componentName: string,
): CardsNewClientProps | null {
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
 * Call the page component with the given searchParams, then traverse the
 * returned JSX tree to find the <CardsNewClient> element and return its props.
 */
async function renderPage(searchParams: Record<string, string> = {}) {
  const result = await CardsNewPage({ searchParams: Promise.resolve(searchParams) });
  const props = findElementProps(result, "CardsNewClient");
  if (!props) {
    throw new Error(
      "CardsNewClient element not found in the page output — page may have redirected",
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

    await expect(
      CardsNewPage({ searchParams: Promise.resolve({}) }),
    ).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("AuthSessionMissingError is silenced → redirect /login, no console.error", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(
      CardsNewPage({ searchParams: Promise.resolve({}) }),
    ).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("non-AuthSessionMissingError → console.error + rethrow", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    await expect(
      CardsNewPage({ searchParams: Promise.resolve({}) }),
    ).rejects.toBe(transportError);

    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[cards-new]"),
      transportError.name,
      transportError.message,
    );
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
      new Error("GraphQL errors: UNAUTHENTICATED"),
    );

    await expect(
      CardsNewPage({ searchParams: Promise.resolve({}) }),
    ).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("non-UNAUTHENTICATED gqlFetch error is rethrown", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(
      CardsNewPage({ searchParams: Promise.resolve({}) }),
    ).rejects.toBe(otherErr);

    expect(redirect).not.toHaveBeenCalled();
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

    await expect(
      CardsNewPage({ searchParams: Promise.resolve({}) }),
    ).rejects.toThrow(`${REDIRECT_PREFIX}/cardgroups/new?welcome=1`);

    expect(redirect).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
  });
});
