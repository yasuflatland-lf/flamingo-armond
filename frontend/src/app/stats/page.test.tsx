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
  StatsClient: ({
    stats,
    performanceWindowsAvailable,
  }: {
    stats: unknown;
    performanceWindowsAvailable?: boolean;
  }) => (
    <div
      data-testid="stats-client"
      data-has-stats={stats != null ? "true" : "false"}
      data-performance-windows-available={String(performanceWindowsAvailable)}
    >
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
          avgDifficulty: 0.5,
        },
        days30: {
          retentionRate: 0.5,
          successRate: 0.5,
          lapseRate: 0.1,
          studyStreak: 0,
          reviewCount: 0,
          avgDifficulty: 0.5,
        },
        days7: {
          retentionRate: 0.5,
          successRate: 0.5,
          lapseRate: 0.1,
          studyStreak: 0,
          reviewCount: 0,
          avgDifficulty: 0.5,
        },
      },
      strugglingCards: [],
    },
  };
}

function makeLegacyStatsData() {
  const { performanceWindows: _performanceWindows, ...stats } = makeStatsData().myLearningStats;
  return {
    myLearningStats: {
      ...stats,
      performance: {
        __typename: "PerformanceMetrics",
        retentionRate: 0.72,
        successRate: 0.8,
        lapseRate: 0.2,
        studyStreak: 4,
        reviewCount: 123,
        avgDifficulty: 0.6,
      },
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

  test("old backend without performanceWindows → retries the legacy query", async () => {
    vi.mocked(gqlFetch)
      .mockRejectedValueOnce(
        new Error(
          `GraphQL errors: ${JSON.stringify([
            {
              message:
                'Cannot query field "performanceWindows" on type "LearningStats". Did you mean "performance"?',
            },
          ])}`,
        ),
      )
      .mockResolvedValueOnce(makeLegacyStatsData() as never);

    const result = await StatsPage();

    expect(gqlFetch).toHaveBeenCalledTimes(2);
    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    const el = findStatsClientElement(result);
    const stats = el?.props.stats as ReturnType<typeof makeStatsData>["myLearningStats"];
    expect(stats.performanceWindows.days365.reviewCount).toBe(123);
    expect(stats.performanceWindows.days30.reviewCount).toBe(123);
    expect(stats.performanceWindows.days7.reviewCount).toBe(123);
    expect(el?.props.performanceWindowsAvailable).toBe(false);
  });

  test("legacy fallback failure → logs and rethrows the fallback error", async () => {
    const fallbackErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch)
      .mockRejectedValueOnce(
        new Error(
          'GraphQL errors: [{"message":"Cannot query field \\"performanceWindows\\" on type \\"LearningStats\\"."}]',
        ),
      )
      .mockRejectedValueOnce(fallbackErr);

    await expect(StatsPage()).rejects.toBe(fallbackErr);
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[stats] legacy gqlFetch failed"),
      { name: "Error" },
    );
  });

  test("legacy resolves with null myLearningStats → clear guard error (not TypeError), no redirect", async () => {
    vi.mocked(gqlFetch)
      .mockRejectedValueOnce(
        new Error(
          `GraphQL errors: ${JSON.stringify([
            { message: 'Cannot query field "performanceWindows" on type "LearningStats".' },
          ])}`,
        ),
      )
      // Partial response: data present but null, errors present but non-auth →
      // gqlFetch RETURNS { myLearningStats: null } instead of throwing.
      .mockResolvedValueOnce({ myLearningStats: null } as never);

    // The null-guard surfaces a clear invariant error, not a raw TypeError from
    // the destructure, and it is rethrown to the error boundary (no redirect).
    await expect(StatsPage()).rejects.toThrow(
      "/stats: legacy myLearningStats returned null with no error",
    );
    expect(gqlFetch).toHaveBeenCalledTimes(2);
    expect(redirect).not.toHaveBeenCalled();
  });

  test("UNAUTHENTICATED from legacy gqlFetch → redirect /login, no console.error", async () => {
    vi.mocked(gqlFetch)
      .mockRejectedValueOnce(
        new Error(
          `GraphQL errors: ${JSON.stringify([
            { message: 'Cannot query field "performanceWindows" on type "LearningStats".' },
          ])}`,
        ),
      )
      .mockRejectedValueOnce(
        new Error(
          `GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`,
        ),
      );

    await expect(StatsPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    // The legacy redirect path swallows the error cleanly — no log.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  test("real gqlparser 'Cannot query field' wording → triggers legacy fallback", async () => {
    // Pin the exact validation wording isPerformanceWindowsUnavailable matches.
    // If gqlparser's phrasing drifts, this fails instead of silently disabling
    // the rollout fallback in production.
    vi.mocked(gqlFetch)
      .mockRejectedValueOnce(
        new Error(
          `GraphQL errors: ${JSON.stringify([
            { message: 'Cannot query field "performanceWindows" on type "LearningStats".' },
          ])}`,
        ),
      )
      .mockResolvedValueOnce(makeLegacyStatsData() as never);

    const result = await StatsPage();

    expect(gqlFetch).toHaveBeenCalledTimes(2);
    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    const el = findStatsClientElement(result);
    expect(el?.props.performanceWindowsAvailable).toBe(false);
  });

  test("validation error matching only some substrings → no legacy fallback, rethrows", async () => {
    // "Cannot query field" matches but "performanceWindows"/"LearningStats" do
    // not → the 3-substring AND must NOT fire the fallback. Locks the predicate
    // against loosening.
    const primaryErr = new Error(
      `GraphQL errors: ${JSON.stringify([
        { message: 'Cannot query field "somethingElse" on type "Query".' },
      ])}`,
    );
    vi.mocked(gqlFetch).mockRejectedValueOnce(primaryErr);

    await expect(StatsPage()).rejects.toBe(primaryErr);
    expect(gqlFetch).toHaveBeenCalledTimes(1);
    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[stats] gqlFetch failed"),
      { name: "Error" },
    );
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

type StatsClientProps = { stats: unknown; performanceWindowsAvailable?: boolean };

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
