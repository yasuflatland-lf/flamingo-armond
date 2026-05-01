// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cardgroupFixture, cardsFixture } from "./fixtures/cardgroups";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// next/navigation — redirect throws so the RSC aborts like Next.js's runtime.
// ---------------------------------------------------------------------------

const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
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

// redirectIfUnauthenticated imports "server-only" (already aliased in
// vitest.config.ts). Mock the module so tests control its behaviour without
// depending on the real navigation module import order.
vi.mock("@/lib/apollo/server-redirect", () => ({
  redirectIfUnauthenticated: vi.fn((err: unknown, target: string): never => {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("UNAUTHENTICATED")) {
      throw new Error(`${REDIRECT_PREFIX}${target}`);
    }
    throw err;
  }),
}));

// Pull mocked symbols AFTER vi.mock registration so vi.mocked resolves them.
import { redirect } from "next/navigation";
import CardgroupDetailPage from "@/app/cardgroups/[id]/page";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeParams(id: string): Promise<{ id: string }> {
  return Promise.resolve({ id });
}

/** Cardgroup response fixture shaped as the GraphQL query result. */
const cardgroupGqlResult = {
  cardgroup: {
    id: cardgroupFixture.id,
    name: cardgroupFixture.name,
    updatedAt: cardgroupFixture.updatedAt,
  },
};

/** Cards response fixture shaped as the GraphQL query result. */
const cardsGqlResult = {
  cardsByCardgroup: cardsFixture.map(({ id, front, back, due, state, cardgroupId }) => ({
    id,
    front,
    back,
    due,
    state,
    cardgroupId,
  })),
};

const emptyCardsGqlResult = { cardsByCardgroup: [] };

/**
 * Stub the two `gqlFetch` calls the page issues in `Promise.all`:
 * [CardgroupQuery, CardsByCardgroupQuery].
 */
function mockDetailPageGql(cardgroupResp: unknown, cardsResp: unknown): void {
  vi.mocked(gqlFetch)
    .mockResolvedValueOnce(cardgroupResp as never)
    .mockResolvedValueOnce(cardsResp as never);
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

beforeEach(() => {
  resetMockSupabase();
  vi.clearAllMocks();
  // Default: logged-in user.
  setMockSupabaseUser({ id: cardgroupFixture.ownerId });
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ===========================================================================
// Test suite
// ===========================================================================

describe("CardgroupDetailPage (broad page-level)", () => {
  // -------------------------------------------------------------------------
  // Case 1: Logged-in user with a valid cardgroup id
  // -------------------------------------------------------------------------
  it("renders the cardgroup name, preview cards, and three CTAs when authenticated", async () => {
    mockDetailPageGql(cardgroupGqlResult, cardsGqlResult);

    const jsx = await CardgroupDetailPage({ params: makeParams(cardgroupFixture.id) });
    render(jsx);

    // Heading reflects the cardgroup name.
    expect(screen.getByRole("heading", { name: cardgroupFixture.name })).toBeInTheDocument();

    // Preview shows the first card's front text.
    // biome-ignore lint/style/noNonNullAssertion: cardsFixture is a literal 5-element array
    expect(screen.getByText(cardsFixture[0]!.front)).toBeInTheDocument();

    // Three action CTAs are present with correct hrefs.
    const id = cardgroupFixture.id;

    const startLearningLink = screen.getByRole("link", { name: /start learning/i });
    expect(startLearningLink).toHaveAttribute("href", `/learn/${id}`);

    const editLink = screen.getByRole("link", { name: /edit/i });
    expect(editLink).toHaveAttribute("href", `/cardgroups/${id}/edit`);

    const manageCardsLink = screen.getByRole("link", { name: /manage cards/i });
    expect(manageCardsLink).toHaveAttribute("href", `/cardgroups/${id}/cards`);

    // No redirect was called.
    expect(redirect).not.toHaveBeenCalled();
  });

  // -------------------------------------------------------------------------
  // Case 2a: Cardgroup not found — gqlFetch returns { cardgroup: null }
  // The page calls redirect("/cardgroups") via the
  // `if (!cardgroupData?.cardgroup) redirect("/cardgroups")` guard.
  // -------------------------------------------------------------------------
  it("redirects to /cardgroups when the cardgroup query returns null", async () => {
    mockDetailPageGql({ cardgroup: null }, emptyCardsGqlResult);

    await expect(CardgroupDetailPage({ params: makeParams("nonexistent-id") })).rejects.toThrow(
      `${REDIRECT_PREFIX}/cardgroups`,
    );

    expect(redirect).toHaveBeenCalledWith("/cardgroups");
  });

  // -------------------------------------------------------------------------
  // Case 2b: Cardgroup not found — gqlFetch returns null data entirely
  // -------------------------------------------------------------------------
  it("redirects to /cardgroups when gqlFetch resolves with null (no cardgroup key)", async () => {
    mockDetailPageGql(null, emptyCardsGqlResult);

    await expect(CardgroupDetailPage({ params: makeParams("missing") })).rejects.toThrow(
      `${REDIRECT_PREFIX}/cardgroups`,
    );

    expect(redirect).toHaveBeenCalledWith("/cardgroups");
  });

  // -------------------------------------------------------------------------
  // Case 3: Logged-out user — Supabase returns user: null
  // The page calls redirect("/login") at the auth gate.
  // -------------------------------------------------------------------------
  it("redirects to /login when no user is authenticated", async () => {
    setMockSupabaseUser(null);

    await expect(CardgroupDetailPage({ params: makeParams(cardgroupFixture.id) })).rejects.toThrow(
      `${REDIRECT_PREFIX}/login`,
    );

    expect(redirect).toHaveBeenCalledWith("/login");
    // gqlFetch must not be called before authentication is confirmed.
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  // -------------------------------------------------------------------------
  // Case 4: No owner check exists in the page source.
  // The page does not inspect the owner and shows the detail to any
  // authenticated user who knows the id — no wrong-owner branch to cover.
  // -------------------------------------------------------------------------

  // -------------------------------------------------------------------------
  // Case 5: Supabase auth transport error — page rethrows without redirecting
  // -------------------------------------------------------------------------
  it("rethrows when Supabase getUser() returns an error", async () => {
    setMockSupabaseUserError(new Error("supabase boom"));

    await expect(CardgroupDetailPage({ params: makeParams(cardgroupFixture.id) })).rejects.toThrow(
      "supabase boom",
    );

    expect(redirect).not.toHaveBeenCalled();
  });

  // -------------------------------------------------------------------------
  // Bonus: empty-cards state renders the "Add card" CTA
  // -------------------------------------------------------------------------
  it("renders the empty-cards state with an Add card link when no cards exist", async () => {
    mockDetailPageGql(cardgroupGqlResult, emptyCardsGqlResult);

    const jsx = await CardgroupDetailPage({ params: makeParams(cardgroupFixture.id) });
    render(jsx);

    expect(screen.getByText(/no cards yet/i)).toBeInTheDocument();

    const addCardLink = screen.getByRole("link", { name: /add card/i });
    expect(addCardLink).toHaveAttribute("href", `/cardgroups/${cardgroupFixture.id}/cards`);
  });
});
