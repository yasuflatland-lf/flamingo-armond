// @vitest-environment happy-dom
/**
 * Broad page-level integration tests for the public catalog deck-detail page
 * (`/catalog/[id]`).
 *
 * Scope: the RSC `CatalogDeckContent` rendered end-to-end through the real
 * CatalogDeckClient + CatalogDeckHeader + ReadOnlyCardRow stack inside a
 * MockedProvider. The `gqlFetch` boundary (SSR seed) is mocked; the client-side
 * Apollo layer seeds itself from the SSR props during render, so the card list
 * renders immediately. A small companion block pins the `/catalog` list "View"
 * affordance to this route — the navigation entry point and its destination are
 * verified together.
 *
 * NOT covered here (owned by narrow tests):
 *   - IntersectionObserver pagination / search debounce → catalog-deck-client.test.tsx
 *   - Import success / error / in-flight branches        → catalog-deck-client.test.tsx
 *   - Badge / description rendering                       → catalog-deck-header.test.tsx
 */

import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
  }),
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
  usePathname: () => "/catalog/deck-1",
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

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({ gqlFetch: vi.fn() }));

import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { NextIntlClientProvider } from "next-intl";
import CatalogDeckPage, { CatalogDeckContent } from "@/app/catalog/[id]/page";
import { CatalogListItem } from "@/app/catalog/catalog-list-item";
import { CatalogDeckFieldsFragment } from "@/app/catalog/queries";
import { makeFragmentData } from "@/generated/fragment-masking";
import { gqlFetch } from "@/lib/apollo/server";
import enMessages from "../messages/en.json";

class FakeIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

const DECK_ID = "deck-1";

const DECK_RESPONSE = {
  masterCardgroup: {
    __typename: "MasterCardgroup" as const,
    id: DECK_ID,
    name: "Business English",
    description: "Professional vocabulary",
  },
};

function masterCard(id: string, front: string, back: string) {
  return { __typename: "MasterCard" as const, id, front, back };
}

function connectionResponse(cards: ReturnType<typeof masterCard>[]) {
  return {
    masterCardsConnection: {
      __typename: "MasterCardConnection" as const,
      edges: cards.map((node) => ({
        __typename: "MasterCardEdge" as const,
        cursor: node.id,
        node,
      })),
      pageInfo: {
        __typename: "PageInfo" as const,
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: cards[0]?.id ?? null,
        endCursor: cards[cards.length - 1]?.id ?? null,
      },
      totalCount: cards.length,
    },
  };
}

const POPULATED = connectionResponse([
  masterCard("mc-1", "hello", "world"),
  masterCard("mc-2", "foo", "bar"),
]);
const EMPTY = connectionResponse([]);

// CatalogDeckContent runs Promise.all([deck, cards]) — the deck fetch is issued
// first, so its mock value is consumed first.
function mockDeckPageGql(
  deck: typeof DECK_RESPONSE | { masterCardgroup: null },
  cards: ReturnType<typeof connectionResponse>,
): void {
  vi.mocked(gqlFetch)
    .mockResolvedValueOnce(deck as never)
    .mockResolvedValueOnce(cards as never);
}

async function renderDeckPage(): Promise<void> {
  const cache = new InMemoryCache();
  const jsx = await CatalogDeckContent({ id: DECK_ID });
  render(
    <MockedProvider mocks={[]} cache={cache}>
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        {jsx as React.ReactElement}
      </NextIntlClientProvider>
    </MockedProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("CatalogDeckPage — broad integration (RSC + deck-detail screen)", () => {
  it("renders the deck name as the h1 title", async () => {
    mockDeckPageGql(DECK_RESPONSE, POPULATED);
    await renderDeckPage();

    expect(screen.getByRole("heading", { level: 1, name: "Business English" })).toBeInTheDocument();
  });

  it("renders the SSR-seeded card fronts/backs via the embedded ReadOnlyCardRow list", async () => {
    mockDeckPageGql(DECK_RESPONSE, POPULATED);
    await renderDeckPage();

    expect(screen.getByText("hello")).toBeInTheDocument();
    expect(screen.getByText("world")).toBeInTheDocument();
    expect(screen.getByText("foo")).toBeInTheDocument();
    expect(screen.getByTestId("catalog-deck-card-list")).toBeInTheDocument();
  });

  it("renders the back link and the import CTA in the header", async () => {
    mockDeckPageGql(DECK_RESPONSE, POPULATED);
    await renderDeckPage();

    expect(screen.getByRole("link", { name: "Back to catalog" })).toHaveAttribute(
      "href",
      "/catalog",
    );
    expect(screen.getByTestId("catalog-deck-import-deck-1")).toBeInTheDocument();
  });

  it("shows the empty-state copy when the deck has no cards", async () => {
    mockDeckPageGql(DECK_RESPONSE, EMPTY);
    await renderDeckPage();

    expect(screen.getByTestId("catalog-deck-empty")).toBeInTheDocument();
    expect(screen.getByText("This deck has no cards yet.")).toBeInTheDocument();
  });

  it("redirects to /login when no user is authenticated", async () => {
    vi.mocked(headers).mockResolvedValueOnce(
      new Headers({ "x-auth-status": "anonymous" }) as never,
    );

    await expect(CatalogDeckPage({ params: Promise.resolve({ id: DECK_ID }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("renders notFound when masterCardgroup is null (unknown or draft id)", async () => {
    mockDeckPageGql({ masterCardgroup: null }, POPULATED);

    await expect(CatalogDeckContent({ id: DECK_ID })).rejects.toThrow("NOT_FOUND");
    expect(notFound).toHaveBeenCalled();
  });

  it("renders notFound when the cards query rejects with BAD_USER_INPUT", async () => {
    const badInput = new Error(
      `GraphQL errors: ${JSON.stringify([
        { message: "draft", extensions: { code: "BAD_USER_INPUT" } },
      ])}`,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(DECK_RESPONSE as never)
      .mockRejectedValueOnce(badInput);

    await expect(CatalogDeckContent({ id: DECK_ID })).rejects.toThrow("NOT_FOUND");
    expect(notFound).toHaveBeenCalled();
  });

  it("redirects to /login when gqlFetch raises an UNAUTHENTICATED GraphQL error", async () => {
    const authErr = new Error(
      `GraphQL errors: ${JSON.stringify([
        { message: "no session", extensions: { code: "UNAUTHENTICATED" } },
      ])}`,
    );
    vi.mocked(gqlFetch).mockRejectedValue(authErr);

    await expect(CatalogDeckContent({ id: DECK_ID })).rejects.toThrow("REDIRECT:/login");
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  it("rethrows a generic (non-auth, non-bad-input) gqlFetch error to the error boundary", async () => {
    const networkErr = new Error("network failure");
    vi.mocked(gqlFetch).mockRejectedValue(networkErr);

    // Neither notFound() nor redirect() — the error propagates unchanged.
    await expect(CatalogDeckContent({ id: DECK_ID })).rejects.toBe(networkErr);
    expect(notFound).not.toHaveBeenCalled();
    expect(redirect).not.toHaveBeenCalled();
  });

  it("throws (not notFound) when masterCardsConnection is null — partial-response null bubble", async () => {
    // The deck resolves fine, but the cards connection comes back null (a partial
    // response delivers the schema-non-null field as null). The guard must throw
    // to the error boundary, not silently render a blank list or call notFound().
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(DECK_RESPONSE as never)
      .mockResolvedValueOnce({ masterCardsConnection: null } as never);

    await expect(CatalogDeckContent({ id: DECK_ID })).rejects.toThrow(
      "masterCardsConnection missing from catalog deck data",
    );
    expect(notFound).not.toHaveBeenCalled();
    expect(redirect).not.toHaveBeenCalled();
  });
});

describe("catalog list → deck-detail entry point", () => {
  it("routes the whole list row to /catalog/{id}", () => {
    const node = makeFragmentData(
      {
        __typename: "MasterCardgroup" as const,
        id: DECK_ID,
        name: "Business English",
        description: null,
        cardCount: 42,
      },
      CatalogDeckFieldsFragment,
    );

    render(
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        <ul>
          <CatalogListItem node={node} />
        </ul>
      </NextIntlClientProvider>,
    );

    expect(screen.getByTestId(`catalog-row-${DECK_ID}`)).toHaveAttribute(
      "href",
      `/catalog/${DECK_ID}`,
    );
  });
});
