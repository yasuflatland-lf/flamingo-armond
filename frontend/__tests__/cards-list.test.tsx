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
import { cardsConnectionFixture, cardsFixture } from "./fixtures/cardgroups";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
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
import { CARDS_PAGE_SIZE } from "@/app/cardgroups/[id]/cards/queries";
import { gqlFetch } from "@/lib/apollo/server";

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

const POPULATED_CONNECTION = cardsConnectionFixture(cardsFixture.slice(0, 3), false, 3);
const EMPTY_CONNECTION = cardsConnectionFixture([], false, 0);

/**
 * Stub the two `gqlFetch` calls the page issues in `Promise.all`:
 * [CardgroupQuery, CardsByCardgroupConnectionQuery].
 */
function mockCardsPageGql(connection: ReturnType<typeof cardsConnectionFixture>): void {
  vi.mocked(gqlFetch)
    .mockResolvedValueOnce(CARDGROUP_RESPONSE as never)
    .mockResolvedValueOnce({ cardsByCardgroupConnection: connection } as never);
}

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
  connection: ReturnType<typeof cardsConnectionFixture>,
  cgId = CG_ID,
): Promise<void> {
  const cache = new InMemoryCache();
  cache.writeQuery({
    query: CardsByCardgroupConnectionDocument,
    variables: { cardgroupId: cgId, first: CARDS_PAGE_SIZE },
    data: { cardsByCardgroupConnection: connection },
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
  // CardsClient directly) and from src/app/cardgroups/[id]/cards/page.test.tsx
  // (which stubs CardsClient).
  it("renders all SSR-seeded card front texts when the user is logged in", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockCardsPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    // Page heading rendered by the RSC layer.
    expect(screen.getByText(`Cards in ${CG_NAME}`)).toBeInTheDocument();

    // Card front texts rendered by CardsClient using the SSR-seeded initialEdges.
    expect(screen.getByText("front-001")).toBeInTheDocument();
    expect(screen.getByText("front-002")).toBeInTheDocument();
    expect(screen.getByText("front-003")).toBeInTheDocument();
  });

  // I2: Empty state — the page renders and CardsClient shows the empty-state copy.
  it("shows empty-state copy when gqlFetch returns an empty connection", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockCardsPageGql(EMPTY_CONNECTION);
    await renderPage(EMPTY_CONNECTION);

    expect(screen.getByText("No cards yet.")).toBeInTheDocument();
  });

  // I3: Supabase auth transport error — page rethrows without redirecting.
  // (redirect and cardgroup-not-found cases are owned by the co-located narrow
  // test at src/app/cardgroups/[id]/cards/page.test.tsx.)
  it("rethrows when Supabase getUser() returns an error", async () => {
    setMockSupabaseUserError(new Error("supabase boom"));

    await expect(CardsPage({ params: Promise.resolve({ id: CG_ID }) })).rejects.toThrow(
      "supabase boom",
    );

    expect(redirect).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });
});
