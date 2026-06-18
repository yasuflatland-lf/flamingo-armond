// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CARDGROUPS_DEFAULT_VARS } from "@/app/cardgroups/queries";
import {
  ImportMasterCardgroupDocument,
  MasterCatalogDocument,
  MyCardgroupsConnectionDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../__tests__/utils/mock-apollo-paginated";
import CatalogClient from "./catalog-client";
import { CATALOG_DEFAULT_VARS } from "./queries";

// ---------------------------------------------------------------------------
// sonner mock — capture the success toast.
// ---------------------------------------------------------------------------
vi.mock("sonner", () => ({
  toast: vi.fn(),
  Toaster: () => null,
}));

import { toast } from "sonner";

// ---------------------------------------------------------------------------
// next/link stub
// ---------------------------------------------------------------------------
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

const M1 = {
  __typename: "MasterCardgroup" as const,
  id: "m-1",
  name: "Business English",
  description: "Professional vocabulary",
  language: "en",
  level: "B2",
  category: "Business",
  cardCount: 42,
};

const M2 = {
  __typename: "MasterCardgroup" as const,
  id: "m-2",
  name: "JLPT N3 Kanji",
  description: null,
  language: "ja",
  level: null,
  category: null,
  cardCount: 100,
};

const M3 = {
  __typename: "MasterCardgroup" as const,
  id: "m-3",
  name: "Travel Phrases",
  description: null,
  language: "en",
  level: "A2",
  category: null,
  cardCount: 30,
};

type MasterNode = typeof M1 | typeof M2 | typeof M3;

function masterEdge(node: MasterNode) {
  return { __typename: "MasterCatalogEdge" as const, cursor: node.id, node };
}

function makeConnection(items: MasterNode[], hasNextPage = false, totalCount?: number) {
  return {
    __typename: "MasterCatalogConnection" as const,
    edges: items.map(masterEdge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: items[0]?.id ?? null,
      endCursor: items[items.length - 1]?.id ?? null,
    },
    totalCount: totalCount ?? items.length,
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
  takeRecords() {
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

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["MasterCatalog", "ImportMasterCardgroup"],
  });
  ioCallbacks = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Render helper — pre-seed the cache so cache-first renders without a network
// round-trip, matching the SSR-seed contract.
// ---------------------------------------------------------------------------

function renderClient(
  mocks: unknown[],
  initialConnection: ReturnType<typeof makeConnection> | null,
  cache: InMemoryCache,
) {
  if (initialConnection) {
    cache.writeQuery({
      query: MasterCatalogDocument,
      variables: CATALOG_DEFAULT_VARS,
      data: { masterCatalog: initialConnection },
    });
  }
  renderWithIntl(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <CatalogClient initialConnection={initialConnection} />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

describe("<CatalogClient>", () => {
  it("renders deck tiles from the SSR seed", async () => {
    const cache = new InMemoryCache();
    renderClient([], makeConnection([M1, M2]), cache);

    expect(await screen.findByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.getByTestId("catalog-import-m-1")).toBeInTheDocument();
    expect(screen.getByTestId("catalog-import-m-2")).toBeInTheDocument();
  });

  it("renders the empty state when no decks are published", async () => {
    const cache = new InMemoryCache();
    renderClient([], makeConnection([]), cache);

    expect(await screen.findByTestId("catalog-empty")).toBeInTheDocument();
  });

  it("imports a deck on success: marks it imported and prepends it to the cardgroups cache", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const importMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "m-1" },
      },
      result: {
        data: {
          importMasterCardgroup: {
            __typename: "ImportMasterCardgroupSuccess",
            cardgroup: {
              __typename: "Cardgroup",
              id: "cg-new",
              name: "Business English",
              updatedAt: "2026-06-13T00:00:00.000Z",
            },
          },
        },
      },
    };

    renderClient([importMock], makeConnection([M1]), cache);

    const importBtn = await screen.findByTestId("catalog-import-m-1");
    await user.click(importBtn);

    // The button flips to the "Imported" affordance and disables.
    await waitFor(() => {
      const btn = screen.getByTestId("catalog-import-m-1");
      expect(btn).toBeDisabled();
      expect(btn).toHaveTextContent("Imported");
    });
    expect(vi.mocked(toast)).toHaveBeenCalledWith('Added "Business English" to your cardgroups.');

    // The imported deck is prepended to the /cardgroups connection cache.
    const snapshot = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    }) as {
      myCardgroupsConnection: { edges: { node: { id: string } }[]; totalCount: number };
    } | null;
    expect(snapshot?.myCardgroupsConnection.edges[0]?.node.id).toBe("cg-new");
    expect(snapshot?.myCardgroupsConnection.totalCount).toBe(1);
  });

  it("surfaces a banner when the import returns MasterNotFoundError", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const importMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "m-1" },
      },
      result: {
        data: {
          importMasterCardgroup: {
            __typename: "MasterNotFoundError",
            message: "master not found",
          },
        },
      },
    };

    renderClient([importMock], makeConnection([M1]), cache);

    await user.click(await screen.findByTestId("catalog-import-m-1"));

    expect(await screen.findByTestId("catalog-import-error")).toHaveTextContent(
      "This cardgroup is no longer available.",
    );
    // The button is NOT marked imported on a not-found outcome.
    expect(screen.getByTestId("catalog-import-m-1")).not.toBeDisabled();
  });

  it("appends the next page when the sentinel intersects", async () => {
    const cache = new InMemoryCache();
    const fetchMoreMock = {
      request: {
        query: MasterCatalogDocument,
        variables: { ...CATALOG_DEFAULT_VARS, after: "m-1", search: null },
      },
      result: { data: { masterCatalog: makeConnection([M2], false, 2) } },
    };

    renderClient([fetchMoreMock], makeConnection([M1], true, 2), cache);

    expect(await screen.findByText("Business English")).toBeInTheDocument();
    fireIntersect();

    expect(await screen.findByText("JLPT N3 Kanji")).toBeInTheDocument();
    // The first page row stays rendered (appended, not replaced).
    expect(screen.getByText("Business English")).toBeInTheDocument();
  });

  it("halts the IO loop and shows a Retry banner when fetchMore fails", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const fetchMoreErrorMock = {
      request: {
        query: MasterCatalogDocument,
        variables: { ...CATALOG_DEFAULT_VARS, after: "m-1", search: null },
      },
      error: new Error("network down"),
    };
    const fetchMoreRetryMock = {
      request: {
        query: MasterCatalogDocument,
        variables: { ...CATALOG_DEFAULT_VARS, after: "m-1", search: null },
      },
      result: { data: { masterCatalog: makeConnection([M2], false, 2) } },
    };

    renderClient([fetchMoreErrorMock, fetchMoreRetryMock], makeConnection([M1], true, 2), cache);

    expect(await screen.findByText("Business English")).toBeInTheDocument();
    fireIntersect();

    const banner = await screen.findByTestId("catalog-fetch-more-error");
    expect(banner).toHaveTextContent("Could not load more cardgroups.");

    // Retry re-runs fetchMore and succeeds.
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("JLPT N3 Kanji")).toBeInTheDocument();
  });

  it("surfaces a banner when the import throws a transport error", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const importErrorMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "m-1" },
      },
      error: new Error("network down"),
    };

    renderClient([importErrorMock], makeConnection([M1]), cache);

    await user.click(await screen.findByTestId("catalog-import-m-1"));

    expect(await screen.findByTestId("catalog-import-error")).toHaveTextContent(
      "Could not import the cardgroup.",
    );
    expect(screen.getByTestId("catalog-import-m-1")).not.toBeDisabled();
  });

  it("debounces search input and refetches with the search variable", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    // Seeded page (search:null) renders from cache; the search query fires the
    // network once the 300ms debounce elapses.
    const searchMock = {
      request: {
        query: MasterCatalogDocument,
        variables: { ...CATALOG_DEFAULT_VARS, search: "kanji" },
      },
      result: { data: { masterCatalog: makeConnection([M2]) } },
    };

    renderClient([searchMock], makeConnection([M1, M3]), cache);

    expect(await screen.findByText("Business English")).toBeInTheDocument();

    // findByText waits past the 300ms debounce (default 1000ms timeout), so real
    // timers stay in play and avoid the fake-timer / MockedProvider flush hazard.
    await user.type(screen.getByTestId("catalog-search"), "kanji");

    expect(await screen.findByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.queryByText("Business English")).not.toBeInTheDocument();
  });

  it("surfaces the sign-in banner when the import fails with UNAUTHENTICATED", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const importAuthMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "m-1" },
      },
      result: {
        errors: [new GraphQLError("unauthenticated", { extensions: { code: "UNAUTHENTICATED" } })],
      },
    };

    renderClient([importAuthMock], makeConnection([M1]), cache);

    await user.click(await screen.findByTestId("catalog-import-m-1"));

    expect(await screen.findByTestId("catalog-import-auth-error")).toBeInTheDocument();
    const signInLink = screen.getByRole("link", { name: /sign in again/i });
    expect(signInLink).toHaveAttribute("href", "/login");
    // The card is NOT marked imported on an auth failure.
    expect(screen.getByTestId("catalog-import-m-1")).not.toBeDisabled();
  });

  it("warns and falls back to a network fetch when the SSR seed is null", async () => {
    const cache = new InMemoryCache();
    // No initialConnection ⇒ no cache seed ⇒ cache-first useQuery must fetch.
    const fetchMock = {
      request: { query: MasterCatalogDocument, variables: CATALOG_DEFAULT_VARS },
      result: { data: { masterCatalog: makeConnection([M1]) } },
    };

    renderClient([fetchMock], null, cache);

    expect(await screen.findByText("Business English")).toBeInTheDocument();
    // The null-seed degradation emits a triage warn (console.warn is the leak spy,
    // which records calls even while swallowing output).
    expect(console.warn).toHaveBeenCalledWith(
      expect.stringContaining("[catalog-client] initialConnection is null"),
    );
  });

  it("serializes imports: a second import click is ignored while one is in flight", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    // Only m-1 has a mock; the serialize guard must keep m-2 from firing a second
    // mutation (an unmatched m-2 request would trip the leak spy in teardown).
    const slowImportMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "m-1" },
      },
      delay: 50,
      result: {
        data: {
          importMasterCardgroup: {
            __typename: "ImportMasterCardgroupSuccess",
            cardgroup: {
              __typename: "Cardgroup",
              id: "cg-new",
              name: "Business English",
              updatedAt: "2026-06-13T00:00:00.000Z",
            },
          },
        },
      },
    };

    renderClient([slowImportMock], makeConnection([M1, M2]), cache);

    await user.click(await screen.findByTestId("catalog-import-m-1"));
    // While m-1 is in flight, clicking m-2 must be a no-op (importingId guard).
    await user.click(screen.getByTestId("catalog-import-m-2"));

    // m-1 eventually completes (delayed mock) and flips to "Imported"; m-2 never imports.
    await waitFor(() => {
      const btn = screen.getByTestId("catalog-import-m-1");
      expect(btn).toBeDisabled();
      expect(btn).toHaveTextContent("Imported");
    });
    expect(screen.getByTestId("catalog-import-m-2")).not.toBeDisabled();
  });

  it("renders the no-match empty state when a search returns no decks", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    const searchMock = {
      request: {
        query: MasterCatalogDocument,
        variables: { ...CATALOG_DEFAULT_VARS, search: "zzz" },
      },
      result: { data: { masterCatalog: makeConnection([]) } },
    };

    renderClient([searchMock], makeConnection([M1]), cache);

    expect(await screen.findByText("Business English")).toBeInTheDocument();
    await user.type(screen.getByTestId("catalog-search"), "zzz");

    expect(await screen.findByTestId("catalog-empty-search")).toBeInTheDocument();
  });

  it("surfaces the error banner when import resolves with an unknown payload variant", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    // A union variant the client was not regenerated against → the hook's
    // unparseable-payload fallthrough returns `rejected`.
    const unknownPayloadMock = {
      request: {
        query: ImportMasterCardgroupDocument,
        variables: { masterCardgroupId: "m-1" },
      },
      result: { data: { importMasterCardgroup: { __typename: "SomeFutureVariant" } } },
    };

    renderClient([unknownPayloadMock], makeConnection([M1]), cache);

    await user.click(await screen.findByTestId("catalog-import-m-1"));

    expect(await screen.findByTestId("catalog-import-error")).toHaveTextContent(
      "Could not import the cardgroup.",
    );
    expect(screen.getByTestId("catalog-import-m-1")).not.toBeDisabled();
  });
});

// ---------------------------------------------------------------------------
// Mobile search takeover — flamingo:open-search opens the bar; the desktop
// input is gated mobile-off so the two do not double up on mobile.
// ---------------------------------------------------------------------------

describe("<CatalogClient> mobile search takeover", () => {
  it("opens the takeover on flamingo:open-search and renders the input", async () => {
    const cache = new InMemoryCache();
    renderClient([], makeConnection([M1]), cache);
    await screen.findByText("Business English");

    expect(screen.queryByTestId("search-takeover")).not.toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new CustomEvent("flamingo:open-search"));
    });

    expect(screen.getByTestId("search-takeover")).toBeInTheDocument();
    expect(screen.getByTestId("search-takeover-input")).toBeInTheDocument();
  });

  it("gates the desktop search input mobile-off (hidden md:block)", async () => {
    const cache = new InMemoryCache();
    renderClient([], makeConnection([M1]), cache);
    await screen.findByText("Business English");

    expect(screen.getByTestId("catalog-search").parentElement).toHaveClass("hidden", "md:block");
  });
});
