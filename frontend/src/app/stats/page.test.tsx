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

// Stub StatsClient — it is a "use client" component with its own dedicated test
// file. The RSC page test only needs to verify the auth/error routing and that
// the fetched stats are forwarded.
vi.mock("./stats-client", () => ({
  StatsClient: ({ stats }: { stats: unknown }) => (
    <div data-testid="stats-client" data-has-stats={stats != null ? "true" : "false"}>
      StatsClient
    </div>
  ),
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import StatsPage from "@/app/stats/page";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeStatsData() {
  return {
    myLearningStats: {
      ownsAnyDeck: true,
      mastery: { inProgress: 1, learned: 2, mature: 3, totalStudied: 6 },
      decks: [],
      performanceWindows: {
        days365: {
          retentionRate: 0.5,
          successRate: 0.5,
          lapseRate: 0.1,
          studyStreak: 0,
          reviewCount: 0,
          knownReviewCount: 0,
          avgDifficulty: 0.5,
        },
        days30: {
          retentionRate: 0.5,
          successRate: 0.5,
          lapseRate: 0.1,
          studyStreak: 0,
          reviewCount: 0,
          knownReviewCount: 0,
          avgDifficulty: 0.5,
        },
        days7: {
          retentionRate: 0.5,
          successRate: 0.5,
          lapseRate: 0.1,
          studyStreak: 0,
          reviewCount: 0,
          knownReviewCount: 0,
          avgDifficulty: 0.5,
        },
      },
      strugglingCards: [],
    },
  };
}

function setAuthStatus(status: string) {
  vi.mocked(headers).mockResolvedValue(
    new Headers({ "x-auth-status": status }) as Awaited<ReturnType<typeof headers>>,
  );
}

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(gqlFetch).mockReset();
  setAuthStatus("authenticated");
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Auth branches
// ---------------------------------------------------------------------------

describe("StatsPage — auth branches", () => {
  test("anonymous → redirect /login, gqlFetch not called", async () => {
    setAuthStatus("anonymous");
    await expect(StatsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("stale → redirect /login, gqlFetch not called", async () => {
    setAuthStatus("stale");
    await expect(StatsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("error status → redirect /login, gqlFetch not called", async () => {
    setAuthStatus("error");
    await expect(StatsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("authenticated → proceeds to gqlFetch", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeStatsData() as never);
    await StatsPage();
    expect(redirect).not.toHaveBeenCalled();
    expect(gqlFetch).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// gqlFetch error branches
// ---------------------------------------------------------------------------

describe("StatsPage — gqlFetch error branches", () => {
  test("UNAUTHENTICATED from gqlFetch → redirect /login, no console.error", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );
    await expect(StatsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    // The redirect path swallows the error cleanly — no log.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  test("non-UNAUTHENTICATED gqlFetch error → console.error (name-only) + rethrow", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);
    await expect(StatsPage()).rejects.toBe(otherErr);
    expect(redirect).not.toHaveBeenCalled();
    // PII redaction: only the error name is logged, never the message/object.
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[stats]"), {
      name: "Error",
    });
  });
});

// ---------------------------------------------------------------------------
// Data guard + happy path
// ---------------------------------------------------------------------------

describe("StatsPage — data guard + happy path", () => {
  test("myLearningStats null → throws server-invariant error", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({ myLearningStats: null } as never);
    await expect(StatsPage()).rejects.toThrow(
      "/stats: myLearningStats returned null with no error",
    );
    expect(redirect).not.toHaveBeenCalled();
  });

  test("authenticated + populated → forwards the fetched stats to StatsClient", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeStatsData() as never);
    const result = await StatsPage();
    expect(redirect).not.toHaveBeenCalled();
    const el = findStatsClientElement(result);
    expect(el).not.toBeNull();
    expect(el?.props.stats).not.toBeNull();
  });
});

// ---------------------------------------------------------------------------
// JSX tree helper
// ---------------------------------------------------------------------------

type StatsClientProps = { stats: unknown };

/**
 * Recursively search a React element tree for the StatsClient stub element so
 * the test can inspect the props forwarded from the page.
 */
function findStatsClientElement(node: unknown): { props: StatsClientProps } | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;

  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string }).name === "StatsClient"
  ) {
    return { props: el.props as StatsClientProps };
  }

  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findStatsClientElement(child);
        if (found) return found;
      }
    } else if (children != null) {
      return findStatsClientElement(children);
    }
  }
  return null;
}
