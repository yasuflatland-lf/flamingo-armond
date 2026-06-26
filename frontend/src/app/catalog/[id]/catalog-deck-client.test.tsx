// @vitest-environment happy-dom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
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

class FakeIntersectionObserver {
  constructor(cb: IntersectionObserverCallback) {
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  if (!cb) return;
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------
beforeEach(() => {
  ioCallbacks = [];
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
    };

    renderClient([fetchMoreMock], makeConnection([C1], true, 2), cache);

    expect(await screen.findByText("hello")).toBeInTheDocument();
    fireIntersect();

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
    const banner = await screen.findByTestId("cards-fetch-more-error");
    expect(banner).toHaveTextContent("Could not reach the server. Please try again.");

    // Retry re-runs fetchMore and succeeds — proves the halt-gate clears.
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("goodbye")).toBeInTheDocument();
  });
});
