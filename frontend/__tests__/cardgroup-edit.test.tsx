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
 *   - Section header link assertions                → cardgroup-cards-section.test.tsx
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

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
  }),
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
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

import { redirect } from "next/navigation";
import { CARDS_PAGE_SIZE } from "@/app/cardgroups/[id]/cards/queries";
import EditCardgroupPage from "@/app/cardgroups/[id]/edit/page";
import { gqlFetch } from "@/lib/apollo/server";

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
    variables: { cardgroupId: cgId, first: CARDS_PAGE_SIZE },
    data: { cardsByCardgroupConnection: connection },
  });

  const jsx = await EditCardgroupPage({ params: Promise.resolve({ id: cgId }) });

  render(
    <MockedProvider mocks={[]} cache={cache}>
      {jsx as React.ReactElement}
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

  it("renders the Settings, Danger zone, and Cards section headings", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    expect(screen.getByRole("heading", { level: 2, name: /^settings$/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: /danger zone/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: /^cards \(/i })).toBeInTheDocument();
  });

  it("renders Start learning and Add card section-header links with correct hrefs", async () => {
    setMockSupabaseUser({ id: "user-admin-1" });

    mockEditPageGql(POPULATED_CONNECTION);
    await renderPage(POPULATED_CONNECTION);

    expect(screen.getByRole("link", { name: /start learning/i })).toHaveAttribute(
      "href",
      `/learn/${CG_ID}`,
    );
    expect(screen.getByRole("link", { name: /add card/i })).toHaveAttribute(
      "href",
      `/cards/new?cardgroup=${CG_ID}&return=/cardgroups/${CG_ID}/edit`,
    );
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
