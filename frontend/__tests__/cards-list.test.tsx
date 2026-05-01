// @vitest-environment jsdom
/**
 * Broad page-level integration tests for the cards list page
 * (`/cardgroups/[id]/cards`).
 *
 * Scope: RSC page rendered end-to-end through the real CardsClient inside a
 * MockedProvider. The gqlFetch boundary (SSR seed) is mocked; the client-side
 * Apollo layer uses a seeded InMemoryCache so CardsClient renders from SSR
 * props immediately.
 *
 * NOT covered here (owned by narrow tests):
 *   - IntersectionObserver pagination / fetchMore  → cards-pagination.test.tsx
 *   - Bulk-delete selection + mutation              → cards-bulk-delete.test.tsx
 */

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";
import {
  cardsConnectionFixture,
  cardsFixture,
} from "./fixtures/cardgroups";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
} from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// Module mocks — must be declared before any imports that trigger them.
// ---------------------------------------------------------------------------

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
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

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Post-mock imports (Vitest hoists vi.mock above imports).
// ---------------------------------------------------------------------------

import { redirect } from "next/navigation";
import CardsPage from "@/app/cardgroups/[id]/cards/page";
import { gqlFetch } from "@/lib/apollo/server";
import { CARDS_PAGE_SIZE } from "@/app/cardgroups/[id]/cards/queries";

// ---------------------------------------------------------------------------
// IntersectionObserver stub — prevents pagination useEffect errors; mirrors
// the minimal shape used in cards-pagination.test.tsx.
// ---------------------------------------------------------------------------

class FakeIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const CG_ID = "cardgroup-001";
const CG_NAME = "Test Cardgroup";

const CARDGROUP_RESPONSE = {
  cardgroup: { id: CG_ID, name: CG_NAME, updatedAt: "2026-01-15T00:00:00Z" },
};

/** Build a minimal populated connection using the shared fixture cards. */
function makeGqlConnectionResponse() {
  const conn = cardsConnectionFixture(cardsFixture.slice(0, 3), false, 3);
  return {
    cardsByCardgroupConnection: conn,
  };
}

const EMPTY_GQL_CONNECTION_RESPONSE = {
  cardsByCardgroupConnection: cardsConnectionFixture([], false, 0),
};

/**
 * Render the RSC page end-to-end: await the async page function, then mount
 * the returned JSX wrapped in a MockedProvider whose cache is pre-seeded with
 * the connection data that CardsClient will read on first render.
 *
 * CardsClient uses fetchPolicy:"cache-first" so it reads immediately from the
 * seeded cache without issuing a network request during the synchronous test
 * phase.
 */
async function renderPage(
  connectionData: Parameters<typeof cardsConnectionFixture>[0] extends never
    ? never
    : { cardsByCardgroupConnection: ReturnType<typeof cardsConnectionFixture> },
  cgId = CG_ID,
) {
  const cache = new InMemoryCache();
  const conn = connectionData.cardsByCardgroupConnection;
  cache.writeQuery({
    query: CardsByCardgroupConnectionDocument,
    variables: { cardgroupId: cgId, first: CARDS_PAGE_SIZE },
    data: { cardsByCardgroupConnection: conn },
  });

  const jsx = await CardsPage({ params: Promise.resolve({ id: cgId }) });

  render(
    <MockedProvider mocks={[]} cache={cache}>
      {jsx as React.ReactElement}
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

beforeEach(() => {
  resetMockSupabase();
  vi.clearAllMocks();
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe("CardsPage — broad integration (RSC + CardsClient)", () => {
  // I1: Logged-in user, populated list — full tree renders all SSR-seeded
  // card front texts. Distinct from cards-pagination.test.tsx (which renders
  // CardsClient directly) and from page.test.tsx (which stubs CardsClient).
  it("renders all SSR-seeded card front texts when the user is logged in", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    const connectionResponse = makeGqlConnectionResponse();

    // gqlFetch is called twice in Promise.all: [CardgroupQuery, CardsByCardgroupConnectionQuery].
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(CARDGROUP_RESPONSE as never)
      .mockResolvedValueOnce(connectionResponse as never);

    await renderPage(connectionResponse);

    // Page heading rendered by the RSC layer.
    expect(screen.getByText(`Cards in ${CG_NAME}`)).toBeInTheDocument();

    // Card front texts rendered by CardsClient using the SSR-seeded initialEdges.
    expect(screen.getByText("front-001")).toBeInTheDocument();
    expect(screen.getByText("front-002")).toBeInTheDocument();
    expect(screen.getByText("front-003")).toBeInTheDocument();
  });

  // I2: Empty state — the page renders and CardsClient shows the empty-state
  // copy from the source ("No cards yet. Add one above.").
  it("shows empty-state copy when gqlFetch returns an empty connection", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(CARDGROUP_RESPONSE as never)
      .mockResolvedValueOnce(EMPTY_GQL_CONNECTION_RESPONSE as never);

    await renderPage(EMPTY_GQL_CONNECTION_RESPONSE);

    expect(screen.getByText("No cards yet. Add one above.")).toBeInTheDocument();
  });

  // I3: Logged-out user — getUser() returns null → redirect("/login").
  it("redirects to /login when the user is not authenticated", async () => {
    setMockSupabaseUser(null);

    await expect(
      CardsPage({ params: Promise.resolve({ id: CG_ID }) }),
    ).rejects.toThrow("REDIRECT:/login");

    expect(redirect).toHaveBeenCalledWith("/login");
    // gqlFetch must NOT be called before the auth gate.
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  // I4: Cardgroup not found — gqlFetch resolves with { cardgroup: null } →
  // page calls redirect("/cardgroups"). Checks the "wrong owner / missing"
  // branch in page.tsx (line: if (!cardgroupData?.cardgroup) redirect("/cardgroups")).
  it("redirects to /cardgroups when the cardgroup is not found", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ cardgroup: null } as never)
      .mockResolvedValueOnce(EMPTY_GQL_CONNECTION_RESPONSE as never);

    await expect(
      CardsPage({ params: Promise.resolve({ id: "nonexistent-cg" }) }),
    ).rejects.toThrow("REDIRECT:/cardgroups");

    expect(redirect).toHaveBeenCalledWith("/cardgroups");
  });
});
