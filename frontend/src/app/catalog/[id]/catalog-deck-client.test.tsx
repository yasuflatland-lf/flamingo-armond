// @vitest-environment happy-dom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  CatalogMasterCardsConnectionDocument,
  ImportMasterCardgroupDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import CatalogDeckClient from "./catalog-deck-client";
import type { CatalogDeck } from "./catalog-deck-header";
import { catalogCardsDefaultVars } from "./queries";

// ---------------------------------------------------------------------------
// Module mocks
// ---------------------------------------------------------------------------
vi.mock("sonner", () => ({ toast: vi.fn(), Toaster: () => null }));

import { toast } from "sonner";

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

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------
const DECK: CatalogDeck = {
  __typename: "MasterCardgroup",
  id: "deck-1",
  name: "Business English",
  description: "Professional vocabulary",
};

const C1 = { __typename: "MasterCard" as const, id: "mc-1", front: "hello", back: "a greeting" };
const C2 = { __typename: "MasterCard" as const, id: "mc-2", front: "goodbye", back: "a farewell" };
const C3 = { __typename: "MasterCard" as const, id: "mc-3", front: "thanks", back: "gratitude" };

type Card = typeof C1 | typeof C2 | typeof C3;

function cardEdge(node: Card) {
  return { __typename: "MasterCardEdge" as const, cursor: node.id, node };
}

function makeConnection(cards: Card[], hasNextPage = false, totalCount?: number) {
  return {
    __typename: "MasterCardConnection" as const,
    edges: cards.map(cardEdge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: cards[0]?.id ?? null,
      endCursor: cards[cards.length - 1]?.id ?? null,
    },
    totalCount: totalCount ?? cards.length,
  };
}

// ---------------------------------------------------------------------------
// IntersectionObserver stub
// ---------------------------------------------------------------------------
let ioCallbacks: IntersectionObserverCallback[] = [];
let ioTargets: Element[] = [];

class FakeIntersectionObserver {
  constructor(cb: IntersectionObserverCallback) {
    ioCallbacks.push(cb);
  }
  observe(target: Element) {
    ioTargets.push(target);
  }
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  // A target inside a `hidden` ancestor is display:none and never intersects in a browser.
  if (!cb || ioTargets.at(-1)?.closest("[hidden]")) return;
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------
beforeEach(() => {
  ioCallbacks = [];
  ioTargets = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Render helper — the client seeds the cache synchronously from props, so a
// fresh cache + no pre-write mirrors the production SSR-seed contract.
// ---------------------------------------------------------------------------
function renderClient(
  mocks: unknown[],
  connection: ReturnType<typeof makeConnection>,
  cache: InMemoryCache,
) {
  renderWithIntl(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <CatalogDeckClient
        id={DECK.id}
        initialDeck={DECK}
        initialEdges={connection.edges}
        initialPageInfo={connection.pageInfo}
        initialTotalCount={connection.totalCount}
      />
    </MockedProvider>,
  );
}

// Seeds a deck, types a search whose query fails with `code`, and returns the
// query-error banner. The default seed is a one-card deck with a next page.
async function renderFailingSearch(
  code: string,
  { connection = makeConnection([C1], true, 2), extraMocks = [] as unknown[] } = {},
) {
  const failingSearchMock = {
    request: {
      query: CatalogMasterCardsConnectionDocument,
      variables: { ...catalogCardsDefaultVars(DECK.id), search: "zzz" },
    },
    result: { errors: [new GraphQLError("boom", { extensions: { code } })] },
  };

  renderClient([failingSearchMock, ...extraMocks], connection, new InMemoryCache());

  const user = userEvent.setup();
  await user.type(await screen.findByTestId("cards-search-input"), "zzz");

  return { banner: await screen.findByTestId("catalog-deck-query-error"), user };
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------
describe("<CatalogDeckClient>", () => {
  it("renders the read-only card rows from the SSR seed", async () => {
    const cache = new InMemoryCache();
    renderClient([], makeConnection([C1, C2]), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    expect(screen.getByText("goodbye")).toBeInTheDocument();
    expect(screen.getByTestId("read-only-card-mc-1")).toBeInTheDocument();
    // No edit chrome leaks through.
    expect(screen.queryByTestId("card-select-mc-1")).toBeNull();
    expect(screen.queryByTestId("card-delete-mc-1")).toBeNull();
  });

  it("renders the empty state when the deck has no cards", async () => {
    const cache = new InMemoryCache();
    renderClient([], makeConnection([]), cache);

    expect(await screen.findByTestId("catalog-deck-empty")).toBeInTheDocument();
  });

  it("appends the next page when the sentinel intersects", async () => {
    const cache = new InMemoryCache();
    const fetchMoreMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), after: "mc-1", search: null },
      },
      result: { data: { masterCardsConnection: makeConnection([C2], false, 2) } },
      delay: 50,
    };

    renderClient([fetchMoreMock], makeConnection([C1], true, 2), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    fireIntersect();

    // The footer stays visible while a page loads (only a search without data hides it).
    expect(await screen.findByTestId("catalog-deck-loading-more")).toBeVisible();
    expect(await screen.findByText("goodbye")).toBeInTheDocument();
    // The first page row stays rendered (appended, not replaced).
    expect(screen.getByText("hello")).toBeInTheDocument();
  });

  it("debounces search input and refetches with the search variable", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    // The seeded page (search:null) renders from cache; the search query fires
    // the network once the 300ms debounce elapses. findByText waits past the
    // debounce on real timers, avoiding the fake-timer / MockedProvider hazard.
    const searchMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), search: "thanks" },
      },
      result: { data: { masterCardsConnection: makeConnection([C3]) } },
    };

    renderClient([searchMock], makeConnection([C1, C2]), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();

    await user.type(screen.getByTestId("cards-search-input"), "thanks");

    expect(await screen.findByText("thanks")).toBeInTheDocument();
    expect(screen.queryByText("hello")).not.toBeInTheDocument();
  });

  it("keeps the import CTA disabled while the import mutation is in flight", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    // delay: Infinity keeps the mutation pending so the in-flight wire is provable.
    const importPendingMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "deck-1" },
      },
      delay: Number.POSITIVE_INFINITY,
      result: {
        data: {
          importMasterCardgroup: {
            __typename: "ImportMasterCardgroupSuccess",
            cardgroup: {
              __typename: "Cardgroup",
              id: "cg-new",
              name: "Business English",
              updatedAt: "2026-06-20T00:00:00.000Z",
            },
          },
        },
      },
    };

    renderClient([importPendingMock], makeConnection([C1]), cache);

    const btn = await screen.findByTestId("catalog-deck-import-deck-1");
    expect(btn).not.toBeDisabled();

    await user.click(btn);

    await waitFor(() => {
      expect(screen.getByTestId("catalog-deck-import-deck-1")).toBeDisabled();
    });
    expect(screen.getByTestId("catalog-deck-import-deck-1")).toHaveTextContent("Importing...");
  });

  it("marks the deck imported and toasts on a successful import", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const importMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "deck-1" },
      },
      result: {
        data: {
          importMasterCardgroup: {
            __typename: "ImportMasterCardgroupSuccess",
            cardgroup: {
              __typename: "Cardgroup",
              id: "cg-new",
              name: "Business English",
              updatedAt: "2026-06-20T00:00:00.000Z",
            },
          },
        },
      },
    };

    renderClient([importMock], makeConnection([C1]), cache);

    await user.click(await screen.findByTestId("catalog-deck-import-deck-1"));

    await waitFor(() => {
      const btn = screen.getByTestId("catalog-deck-import-deck-1");
      expect(btn).toBeDisabled();
      expect(btn).toHaveTextContent("Imported");
    });
    expect(vi.mocked(toast)).toHaveBeenCalledWith('Added "Business English" to your cardgroups.');
  });

  it("shows the error banner when the import returns MasterNotFoundError", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const notFoundMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "deck-1" },
      },
      result: {
        data: { importMasterCardgroup: { __typename: "MasterNotFoundError", message: "gone" } },
      },
    };

    renderClient([notFoundMock], makeConnection([C1]), cache);

    await user.click(await screen.findByTestId("catalog-deck-import-deck-1"));

    expect(await screen.findByTestId("catalog-deck-import-error")).toHaveTextContent(
      "This cardgroup is no longer available.",
    );
    // Not marked imported on a not-found outcome.
    expect(screen.getByTestId("catalog-deck-import-deck-1")).not.toBeDisabled();
  });

  it("shows the limit banner when the import returns CardgroupLimitReachedError", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const limitMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "deck-1" },
      },
      result: {
        data: {
          importMasterCardgroup: {
            __typename: "CardgroupLimitReachedError",
            message: "cardgroup limit reached",
            limit: 5,
            current: 5,
          },
        },
      },
    };

    renderClient([limitMock], makeConnection([C1]), cache);

    await user.click(await screen.findByTestId("catalog-deck-import-deck-1"));

    // Reuses the create-form copy from the Cardgroups namespace, interpolated
    // with the backend-supplied counts.
    expect(await screen.findByTestId("catalog-deck-import-error")).toHaveTextContent(
      "You already have 5 card groups (maximum 5).",
    );
    // Not marked imported on a limit-reached outcome.
    expect(screen.getByTestId("catalog-deck-import-deck-1")).not.toBeDisabled();
  });

  it("shows the generic error banner when the import throws a transport error", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const rejectedMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "deck-1" },
      },
      error: new Error("network down"),
    };

    renderClient([rejectedMock], makeConnection([C1]), cache);

    await user.click(await screen.findByTestId("catalog-deck-import-deck-1"));

    expect(await screen.findByTestId("catalog-deck-import-error")).toHaveTextContent(
      "Could not import the cardgroup. Please try again.",
    );
    expect(screen.getByTestId("catalog-deck-import-deck-1")).not.toBeDisabled();
  });

  it("shows the sign-in banner when the import fails with UNAUTHENTICATED", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const authMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "deck-1" },
      },
      result: {
        errors: [new GraphQLError("unauthenticated", { extensions: { code: "UNAUTHENTICATED" } })],
      },
    };

    renderClient([authMock], makeConnection([C1]), cache);

    await user.click(await screen.findByTestId("catalog-deck-import-deck-1"));

    expect(await screen.findByTestId("catalog-deck-import-auth-error")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Sign in again" })).toHaveAttribute("href", "/login");
  });

  it("renders the empty-search state when a search returns no cards", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const emptySearchMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), search: "zzz" },
      },
      result: { data: { masterCardsConnection: makeConnection([]) } },
    };

    renderClient([emptySearchMock], makeConnection([C1]), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    await user.type(screen.getByTestId("cards-search-input"), "zzz");

    expect(await screen.findByTestId("catalog-deck-empty-search")).toBeInTheDocument();
    expect(screen.queryByTestId("catalog-deck-empty")).toBeNull();
  });

  it("renders the query-error banner and hides the unfiltered SSR list when a search query fails", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const failingSearchMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), search: "zzz" },
      },
      result: { errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })] },
    };

    renderClient([failingSearchMock], makeConnection([C1]), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    await user.type(screen.getByTestId("cards-search-input"), "zzz");

    expect(await screen.findByTestId("catalog-deck-query-error")).toHaveTextContent("boom");
    expect(screen.queryByTestId("catalog-deck-card-list")).toBeNull();
    expect(screen.queryByTestId("catalog-deck-empty-search")).toBeNull();
    expect(screen.queryByTestId("catalog-deck-empty")).toBeNull();
    expect(screen.queryByText("hello")).toBeNull();
  });

  it("renders only the query-error banner, not the no-match state, when a search on an empty deck fails", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const failingSearchMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), search: "zzz" },
      },
      result: { errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })] },
    };

    renderClient([failingSearchMock], makeConnection([]), cache);

    expect(await screen.findByTestId("catalog-deck-empty")).toBeInTheDocument();
    await user.type(screen.getByTestId("cards-search-input"), "zzz");

    expect(await screen.findByTestId("catalog-deck-query-error")).toHaveTextContent("boom");
    expect(screen.queryByTestId("catalog-deck-empty-search")).toBeNull();
    expect(screen.queryByTestId("catalog-deck-empty")).toBeNull();
  });

  it("hides the footer during a failed search and recovers via Retry with the sentinel still observed", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const searchVars = { ...catalogCardsDefaultVars(DECK.id), search: "zzz" };
    const failingSearchMock = {
      request: { query: CatalogMasterCardsConnectionDocument, variables: searchVars },
      result: { errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })] },
    };
    // Same edge count and hasNextPage as the SSR seed, so the observer effect deps stay unchanged.
    const recoveredSearchMock = {
      request: { query: CatalogMasterCardsConnectionDocument, variables: searchVars },
      result: { data: { masterCardsConnection: makeConnection([C3], true, 2) } },
    };

    renderClient([failingSearchMock, recoveredSearchMock], makeConnection([C1], true, 2), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    await user.type(screen.getByTestId("cards-search-input"), "zzz");

    const banner = await screen.findByTestId("catalog-deck-query-error");
    expect(screen.queryByTestId("catalog-deck-card-list")).toBeNull();
    expect(screen.getByTestId("catalog-deck-sentinel")).not.toBeVisible();

    await user.click(within(banner).getByRole("button", { name: "Retry" }));

    expect(await screen.findByText("thanks")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByTestId("catalog-deck-query-error")).toBeNull());
    const sentinel = screen.getByTestId("catalog-deck-sentinel");
    expect(sentinel).toBeVisible();
    expect(ioTargets.at(-1)).toBe(sentinel);
    expect(ioTargets.at(-1)?.isConnected).toBe(true);
  });

  it("keeps the footer hidden and never paginates the SSR cursor while a Retry is in flight", async () => {
    const searchVars = { ...catalogCardsDefaultVars(DECK.id), search: "zzz" };
    // delay: Infinity keeps the Retry pending so the in-flight window is observable.
    const pendingRetryMock = {
      request: { query: CatalogMasterCardsConnectionDocument, variables: searchVars },
      result: { data: { masterCardsConnection: makeConnection([C3], true, 2) } },
      delay: Infinity,
    };
    const staleCursorResult = vi.fn(() => ({
      data: { masterCardsConnection: makeConnection([C2], false, 2) },
    }));
    const staleCursorMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...searchVars, after: "mc-1" },
      },
      result: staleCursorResult,
    };

    const { banner, user } = await renderFailingSearch("INTERNAL", {
      extraMocks: [pendingRetryMock, staleCursorMock],
    });
    await user.click(within(banner).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByTestId("catalog-deck-query-error")).toBeNull());

    // The rendered page is still the SSR fallback, so its endCursor must not be paginated.
    expect(screen.getByTestId("catalog-deck-sentinel")).not.toBeVisible();
    fireIntersect();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(staleCursorResult).not.toHaveBeenCalled();
  });

  it("keeps the footer hidden and never paginates the SSR cursor while an edited search term loads", async () => {
    const searchVars = { ...catalogCardsDefaultVars(DECK.id), search: "zzz" };
    const pendingSearchMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...searchVars, search: "zzzz" },
      },
      result: { data: { masterCardsConnection: makeConnection([C3], true, 2) } },
      delay: Infinity,
    };
    const staleCursorResult = vi.fn(() => ({
      data: { masterCardsConnection: makeConnection([C2], false, 2) },
    }));
    const staleCursorMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...searchVars, search: "zzzz", after: "mc-1" },
      },
      result: staleCursorResult,
    };

    const { user } = await renderFailingSearch("INTERNAL", {
      extraMocks: [pendingSearchMock, staleCursorMock],
    });

    await user.type(screen.getByTestId("cards-search-input"), "z");
    await waitFor(() => expect(screen.queryByTestId("catalog-deck-query-error")).toBeNull());

    expect(screen.getByTestId("catalog-deck-sentinel")).not.toBeVisible();
    fireIntersect();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(staleCursorResult).not.toHaveBeenCalled();
  });

  it("renders the sign-in banner and hides the list and footer when a search fails with UNAUTHENTICATED", async () => {
    const { banner } = await renderFailingSearch("UNAUTHENTICATED");

    expect(banner).toHaveTextContent("Your session has expired.");
    expect(within(banner).getByRole("link", { name: "Sign in again" })).toHaveAttribute(
      "href",
      "/login",
    );
    expect(within(banner).queryByRole("button", { name: "Retry" })).toBeNull();
    expect(screen.queryByTestId("catalog-deck-card-list")).toBeNull();
    expect(screen.queryByTestId("catalog-deck-empty-search")).toBeNull();
    expect(screen.getByTestId("catalog-deck-sentinel")).not.toBeVisible();
  });

  it("suppresses the no-match state on an empty deck when a search fails with UNAUTHENTICATED", async () => {
    const { banner } = await renderFailingSearch("UNAUTHENTICATED", {
      connection: makeConnection([]),
    });

    expect(banner).toHaveTextContent("Your session has expired.");
    expect(screen.queryByTestId("catalog-deck-empty-search")).toBeNull();
    expect(screen.queryByTestId("catalog-deck-empty")).toBeNull();
  });

  it("renders the permission banner without Retry and hides the list and footer when a search fails with FORBIDDEN", async () => {
    const { banner } = await renderFailingSearch("FORBIDDEN");

    expect(banner).toHaveTextContent("You do not have permission.");
    expect(within(banner).queryByRole("button", { name: "Retry" })).toBeNull();
    expect(screen.queryByTestId("catalog-deck-card-list")).toBeNull();
    expect(screen.queryByTestId("catalog-deck-empty-search")).toBeNull();
    expect(screen.getByTestId("catalog-deck-sentinel")).not.toBeVisible();
  });

  it("halts the IO loop and shows a Retry banner when fetchMore fails", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const fetchMoreErrorMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), after: "mc-1", search: null },
      },
      error: new Error("network down"),
    };
    const fetchMoreRetryMock = {
      request: {
        query: CatalogMasterCardsConnectionDocument,
        variables: { ...catalogCardsDefaultVars(DECK.id), after: "mc-1", search: null },
      },
      result: { data: { masterCardsConnection: makeConnection([C2], false, 2) } },
    };

    renderClient([fetchMoreErrorMock, fetchMoreRetryMock], makeConnection([C1], true, 2), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    fireIntersect();

    // A transport error is mapped by getBackendErrorBanner (the localized
    // fetchMoreErrorMessage fallback only fires when that mapping returns undefined).
    const banner = await screen.findByTestId("catalog-deck-fetch-more-error");
    expect(banner).toHaveTextContent("Could not reach the server. Please try again.");

    // Retry re-runs fetchMore and succeeds — proves the halt-gate clears.
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("goodbye")).toBeInTheDocument();
  });
});
