import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({ gqlFetch: vi.fn() }));

// Stub the client — the RSC test only verifies the render branch.
vi.mock("./onboarding-start-client", () => ({
  OnboardingStartClient: () => <div data-testid="onboarding-start-client" />,
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import OnboardingStartPage from "@/app/onboarding/start/page";
import { gqlFetch } from "@/lib/apollo/server";

type DeckNode = {
  __typename: "MasterCardgroup";
  id: string;
  name: string;
  description: string | null;
  language: string | null;
  level: string | null;
  category: string | null;
  cardCount: number;
};

function makeData(
  opts: { displayName?: string | null; totalCount?: number; nodes?: DeckNode[] } = {},
) {
  const nodes: DeckNode[] = opts.nodes ?? [
    {
      __typename: "MasterCardgroup",
      id: "m-1",
      name: "Business English",
      description: null,
      language: "en",
      level: "B2",
      category: null,
      cardCount: 42,
    },
  ];
  return {
    me: { id: "u-1", displayName: opts.displayName !== undefined ? opts.displayName : "Alice" },
    masterCatalog: {
      __typename: "MasterCatalogConnection",
      edges: nodes.map((n) => ({ __typename: "MasterCatalogEdge", cursor: n.id, node: n })),
      totalCount: opts.totalCount ?? nodes.length,
      pageInfo: {
        __typename: "PageInfo",
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: nodes[0]?.id ?? null,
        endCursor: nodes[nodes.length - 1]?.id ?? null,
      },
    },
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "authenticated" }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("OnboardingStartPage — auth", () => {
  test("anonymous → /login, gqlFetch not called", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "anonymous" }));

    await expect(OnboardingStartPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });
});

describe("OnboardingStartPage — gqlFetch errors", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  test("UNAUTHENTICATED from gqlFetch → /login", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(OnboardingStartPage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  test("non-UNAUTHENTICATED error → console.error + rethrow", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(OnboardingStartPage()).rejects.toBe(otherErr);
    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[onboarding-start]"),
      expect.anything(),
    );
  });
});

describe("OnboardingStartPage — guards & fallback", () => {
  test("not onboarded (displayName null) → /onboarding", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeData({ displayName: null }) as never);

    await expect(OnboardingStartPage()).rejects.toThrow(`${REDIRECT_PREFIX}/onboarding`);
    expect(redirect).toHaveBeenCalledWith("/onboarding");
  });

  test("empty catalog (totalCount 0) → /cardgroups/new?welcome=1", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeData({ totalCount: 0, nodes: [] }) as never);

    await expect(OnboardingStartPage()).rejects.toThrow(
      `${REDIRECT_PREFIX}/cardgroups/new?welcome=1`,
    );
    expect(redirect).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
  });

  test("null masterCatalog → console.error + throws", async () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: "Alice" },
      masterCatalog: null,
    } as never);

    await expect(OnboardingStartPage()).rejects.toThrow(/masterCatalog missing/);
    expect(spy).toHaveBeenCalledWith(expect.stringContaining("[onboarding-start]"));
  });
});

describe("OnboardingStartPage — render", () => {
  test("populated catalog renders the client inside a <main> landmark", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeData({ displayName: "Alice" }) as never);

    const result = await OnboardingStartPage();

    expect(redirect).not.toHaveBeenCalled();
    expect((result as { type?: unknown }).type).toBe("main");
  });
});
