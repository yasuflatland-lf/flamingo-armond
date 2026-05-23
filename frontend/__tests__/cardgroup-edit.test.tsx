// @vitest-environment jsdom
/**
 * Broad page-level integration tests for the integrated cardgroup management
 * page (`/cardgroups/[id]/edit`).
 *
 * Scope: RSC page rendered end-to-end through the real
 * CardgroupManagementClient + CardgroupSettingsCard + CardgroupCardsSection
 * + CardsClient stack inside a MockedProvider. The gqlFetch boundary (SSR
 * seed) is mocked; the client-side Apollo layer uses a seeded InMemoryCache
 * so CardsClient renders from SSR props immediately.
 *
 * NOT covered here (owned by narrow tests):
 *   - IntersectionObserver pagination / fetchMore  → cards-pagination.test.tsx
 *   - Bulk-delete selection + mutation              → cards-bulk-delete.test.tsx
 *   - Mutation success/error branches               → cardgroup-settings-card.test.tsx
 *   - Section header control assertions             → cardgroup-cards-section.test.tsx
 */

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";
import { cardsConnectionFixture, cardsFixture } from "./fixtures/cardgroups";
import {
  mockCreateSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
  }),
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
  // CardsClient (rendered via the management screen) reads usePathname for
  // its pending-delete flush effect. The integration test does not exercise
  // navigation transitions, so a stable stub value is sufficient.
  usePathname: () => "/cardgroups/cg-int-1/edit",
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
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { redirect } from "next/navigation";
import { cardsDefaultVars } from "@/app/cardgroups/[id]/cards/queries";
import EditCardgroupPage from "@/app/cardgroups/[id]/edit/page";
import { gqlFetch } from "@/lib/apollo/server";
import { UndoDeleteProvider } from "@/lib/undo-delete";

class FakeIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

const CG_ID = "cardgroup-001";
const CG_NAME = "Test Cardgroup";

const CARDGROUP_RESPONSE = {
  cardgroup: { id: CG_ID, name: CG_NAME, updatedAt: "2026-01-15T00:00:00Z" },
};

const POPULATED_CONNECTION = cardsConnectionFixture(cardsFixture.slice(0, 3), false, 3);
const EMPTY_CONNECTION = cardsConnectionFixture([], false, 0);

function mockEditPageGql(connection: ReturnType<typeof cardsConnectionFixture>): void {
  vi.mocked(gqlFetch)
    .mockResolvedValueOnce(CARDGROUP_RESPONSE as never)
    .mockResolvedValueOnce({ cardsByCardgroupConnection: connection } as never);
}

async function renderPage(
  connection: ReturnType<typeof cardsConnectionFixture>,
  cgId = CG_ID,
): Promise<void> {
  const cache = new InMemoryCache();
  cache.writeQuery({
    query: CardsByCardgroupConnectionDocument,
    // Variables shape MUST match cardsDefaultVars(cgId) — cardgroupId, first,
    // and `search: null`. Omitting `search` silently splits the cache key and
    // the CardsClient's useQuery returns undefined (falling back to initialEdges).
    variables: cardsDefaultVars(cgId),
    data: { cardsByCardgroupConnection: connection },
  });

  const jsx = await EditCardgroupPage({ params: Promise.resolve({ id: cgId }) });

  render(
    <MockedProvider mocks={[]} cache={cache}>
      <UndoDeleteProvider>{jsx as React.ReactElement}</UndoDeleteProvider>
    </MockedProvider>,
  );
}

beforeEach(() => {
  resetMockSupabase();
  vi.clearAllMocks();
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("EditCardgroupPage — broad integration (RSC + management screen)", () => {
  it("renders the cardgroup name as the h1 page title", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    expect(screen.getByRole("heading", { level: 1, name: CG_NAME })).toBeInTheDocument();
  });

  it("renders all SSR-seeded card front texts via the embedded CardsClient", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    expect(screen.getByText("front-001")).toBeInTheDocument();
    expect(screen.getByText("front-002")).toBeInTheDocument();
    expect(screen.getByText("front-003")).toBeInTheDocument();
  });

  it("renders the kebab button and card count Badge in the page header", async () => {
    // Settings and Danger zone were moved into the kebab DropdownMenu (Task 1).
    // The count chip was removed from the toolbar (Task 2); count shows in the
    // page-level Badge next to the h1 instead.
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    // The kebab trigger button is present.
    expect(screen.getByRole("button", { name: /cardgroup options/i })).toBeInTheDocument();
    // The Badge with the card count is present (3 cards in POPULATED_CONNECTION).
    expect(screen.getByText("3 cards")).toBeInTheDocument();
  });

  it("renders Start learning link and opens the in-context Add card sheet", async () => {
    const user = userEvent.setup();
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    expect(screen.getByRole("link", { name: /start learning/i })).toHaveAttribute(
      "href",
      `/learn/${CG_ID}`,
    );
    await user.click(screen.getByRole("button", { name: /add card/i }));
    expect(screen.getByRole("heading", { name: /add card/i })).toBeInTheDocument();
  });

  it("shows the empty-state copy when gqlFetch returns an empty connection", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(EMPTY_CONNECTION);
    await renderPage(EMPTY_CONNECTION);

    expect(screen.getByText("Add some new cards to get started.")).toBeInTheDocument();
  });

  it("redirects to /login when no user is authenticated", async () => {
    setMockSupabaseUser(null);

    await expect(EditCardgroupPage({ params: Promise.resolve({ id: CG_ID }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("rethrows when Supabase getUser() returns an error", async () => {
    setMockSupabaseUserError(new Error("supabase boom"));

    await expect(EditCardgroupPage({ params: Promise.resolve({ id: CG_ID }) })).rejects.toThrow(
      "supabase boom",
    );

    expect(redirect).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });
});
