// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  DeleteCardDocument,
  UpdateCardDocument,
} from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../../__tests__/utils/mock-apollo-paginated";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// Mock sonner so the Snackbar's Undo action callback can be invoked
// programmatically. This mirrors the pattern in undo-delete.test.ts.
let lastUndoAction: (() => void) | undefined;
vi.mock("sonner", () => ({
  toast: vi.fn((_label: string, opts?: { action?: { onClick?: () => void } }) => {
    lastUndoAction = opts?.action?.onClick;
  }),
}));

// usePathname can be flipped per-test via mockUsePathname.
const mockUsePathname = vi.fn(() => "/cardgroups/cg-1/edit");
vi.mock("next/navigation", () => ({
  usePathname: () => mockUsePathname(),
}));

// next/link → plain anchor in jsdom.
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

import { CardsClient } from "./cards-client";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_ID = "cg-1";
const PAGE_SIZE = 20;

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  due: "2024-06-15",
  state: 0,
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  due: "2024-06-15",
  state: 0,
  cardgroupId: CG_ID,
};

function edge(card: typeof CARD_1) {
  return {
    __typename: "CardEdge" as const,
    cursor: card.id,
    node: card,
  };
}

function connection(
  cards: ReadonlyArray<typeof CARD_1>,
  hasNext = false,
): {
  __typename: "CardConnection";
  edges: ReturnType<typeof edge>[];
  pageInfo: {
    __typename: "PageInfo";
    hasNextPage: boolean;
    hasPreviousPage: boolean;
    startCursor: string | null;
    endCursor: string | null;
  };
  totalCount: number;
} {
  return {
    __typename: "CardConnection" as const,
    edges: cards.map(edge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage: hasNext,
      hasPreviousPage: false,
      startCursor: cards[0]?.id ?? null,
      endCursor: cards[cards.length - 1]?.id ?? null,
    },
    totalCount: cards.length,
  };
}

function defaultPageInfo(edges: ReturnType<typeof edge>[]) {
  return {
    __typename: "PageInfo" as const,
    hasNextPage: false,
    hasPreviousPage: false,
    startCursor: edges[0]?.cursor ?? null,
    endCursor: edges[edges.length - 1]?.cursor ?? null,
  };
}

// Default vars shape — must match cardsDefaultVars(CG_ID).
const DEFAULT_VARS = { cardgroupId: CG_ID, first: PAGE_SIZE, search: null };

function makeUpdateMock(id: string, input: { front: string; back: string }, card = CARD_1) {
  return {
    request: { query: UpdateCardDocument, variables: { id, input } },
    result: {
      data: {
        updateCard: { __typename: "UpdateCardPayload" as const, card },
      },
    },
  };
}

function renderClient(
  mocks: unknown[],
  initialCards = [CARD_1, CARD_2],
  options: { errorPolicy?: boolean; cache?: InMemoryCache; skipSeed?: boolean } = {},
) {
  const defaultOptions = options.errorPolicy
    ? { mutate: { errorPolicy: "all" as const } }
    : undefined;

  const initialEdges = initialCards.map(edge);

  // Seed an InMemoryCache with the initial connection by default. The leak spy
  // catches any unmatched useQuery network call, so every test must either
  // (a) provide a cache that already contains DEFAULT_VARS-keyed data, or
  // (b) supply a matching MockedResponse for the initial query, or
  // (c) opt out via skipSeed when the test deliberately exercises a query
  // error path with its own mock.
  const cache =
    options.cache ??
    (() => {
      if (options.skipSeed) return new InMemoryCache();
      const c = new InMemoryCache();
      c.writeQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: DEFAULT_VARS,
        data: { cardsByCardgroupConnection: connection(initialCards) },
      });
      return c;
    })();

  render(
    <MockedProvider mocks={mocks as never} defaultOptions={defaultOptions} cache={cache}>
      <CardsClient
        cardgroupId={CG_ID}
        initialEdges={initialEdges}
        initialPageInfo={defaultPageInfo(initialEdges)}
        initialTotalCount={initialEdges.length}
      />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// IntersectionObserver stub — capture the most recent observer's callback so
// individual tests can fire intersection events.
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
  leakSpy = installApolloMockLeakSpy({ operationNames: ["CardsByCardgroupConnection"] });
  ioCallbacks = [];
  lastUndoAction = undefined;
  mockUsePathname.mockReturnValue("/cardgroups/cg-1/edit");
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
// Tests
// ---------------------------------------------------------------------------

describe("<CardsClient>", () => {
  it("renders existing cards", () => {
    renderClient([]);
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Bye")).toBeInTheDocument();
  });

  it("does not render a create form (Add a card heading absent)", () => {
    renderClient([]);
    expect(screen.queryByText(/add a card/i)).toBeNull();
    expect(screen.queryByRole("button", { name: /^add$/i })).toBeNull();
  });

  // Task 6: row click enters edit mode
  it("clicking the row text enters edit mode and Cancel reverts to view", async () => {
    const user = userEvent.setup();
    renderClient([]);

    await user.click(screen.getByTestId("card-edit-target-c-1"));

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    expect(editFrontInput).toHaveValue("Hello");

    await user.click(screen.getByRole("button", { name: /cancel/i }));

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /cancel/i })).not.toBeInTheDocument();
    });
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });

  // Task 6: keyboard activates edit mode (Enter key)
  it("keyboard Enter on the row text enters edit mode", async () => {
    const user = userEvent.setup();
    renderClient([]);

    const target = screen.getByTestId("card-edit-target-c-1");
    target.focus();
    await user.keyboard("{Enter}");

    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
  });

  // Task 6 regression: clicking the checkbox does NOT enter edit mode.
  it("clicking the checkbox toggles selection without entering edit mode", async () => {
    const user = userEvent.setup();
    renderClient([]);

    await user.click(screen.getByTestId("card-select-c-1"));

    expect(screen.getByTestId("cards-bulk-action-bar")).toBeInTheDocument();
    expect(screen.queryByLabelText(/^front$/i)).toBeNull();
  });

  // Task 6 regression: clicking the Delete icon does NOT enter edit mode (and
  // also does not surface an AlertDialog — Task 5b removed it).
  it("clicking the Delete icon does not enter edit mode and does not open AlertDialog", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([], [CARD_1, CARD_2], { cache });

    await user.click(screen.getByTestId("card-delete-c-1"));

    // Edit mode not entered.
    expect(screen.queryByLabelText(/^front$/i)).toBeNull();
    // No AlertDialog confirmation appears for per-row delete (Task 5b).
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("edit save calls updateCard mutation and shows updated values", async () => {
    const user = userEvent.setup();
    const updatedCard = { ...CARD_1, front: "Hello updated", back: "Hola updated" };
    const mock = makeUpdateMock(
      "c-1",
      { front: "Hello updated", back: "Hola updated" },
      updatedCard,
    );

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([mock], [CARD_1, CARD_2], { cache });

    await user.click(screen.getByTestId("card-edit-target-c-1"));

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);
    await user.type(editFrontInput, "Hello updated");

    const editBackInput = screen.getByLabelText(/back/i) as HTMLElement;
    await user.clear(editBackInput);
    await user.type(editBackInput, "Hola updated");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("Hello updated")).toBeInTheDocument();
    });
  });

  it("edit save failure keeps the row in edit mode and shows the inline error", async () => {
    const user = userEvent.setup();

    const mock = {
      request: {
        query: UpdateCardDocument,
        variables: { id: "c-1", input: { front: "", back: "Hola" } },
      },
      result: {
        errors: [
          new GraphQLError("front is required", {
            extensions: { code: "BAD_USER_INPUT", field: "front" },
          }),
        ],
      },
    };

    renderClient([mock], [CARD_1], { errorPolicy: true });

    await user.click(screen.getByTestId("card-edit-target-c-1"));

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("front is required")).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
  });

  // Task 5b: per-row delete via scheduleDelete — row disappears immediately,
  // 5-second timer fires real DELETE mutation.
  it("delete via Snackbar: row disappears immediately, mutation fires after 5s", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    let mutationFired = false;
    const deleteMock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: () => {
        mutationFired = true;
        return { data: { deleteCard: true } };
      },
    };

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([deleteMock], [CARD_1, CARD_2], { cache });

    await user.click(screen.getByTestId("card-delete-c-1"));

    // Optimistic remove must be immediate.
    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });
    expect(mutationFired).toBe(false);

    // Advance past the 5-second undo window.
    await vi.advanceTimersByTimeAsync(5000);

    await waitFor(() => {
      expect(mutationFired).toBe(true);
    });
  });

  // Task 5b: Undo within 5s restores the row and prevents the DELETE mutation.
  it("delete then Undo within 5s restores the row and does NOT fire DELETE", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    let mutationFired = false;
    const deleteMock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: () => {
        mutationFired = true;
        return { data: { deleteCard: true } };
      },
    };

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([deleteMock], [CARD_1, CARD_2], { cache });

    await user.click(screen.getByTestId("card-delete-c-1"));

    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });

    // Click Undo before the timer elapses.
    expect(lastUndoAction).toBeDefined();
    if (lastUndoAction) lastUndoAction();

    // Advance past the original window — mutation must NOT fire because the
    // timer was cancelled.
    await vi.advanceTimersByTimeAsync(6000);

    expect(mutationFired).toBe(false);

    // Row should be back.
    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });
    // Note: deleteMock is intentionally unconsumed here. MockedProvider does
    // not warn about unused mocks, only about unmatched requests, so the leak
    // spy stays clean and the unused entry is harmless.
  });

  // Task 5b: pathname change triggers flushPendingDeletes — DELETE fires
  // immediately even though the 5s window has not elapsed.
  it("pathname change flushes pending delete, firing DELETE immediately", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    let mutationFired = false;
    const deleteMock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: () => {
        mutationFired = true;
        return { data: { deleteCard: true } };
      },
    };

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    const initialEdges = [CARD_1, CARD_2].map(edge);

    function Harness() {
      return (
        <CardsClient
          cardgroupId={CG_ID}
          initialEdges={initialEdges}
          initialPageInfo={defaultPageInfo(initialEdges)}
          initialTotalCount={initialEdges.length}
        />
      );
    }

    const { rerender } = render(
      <MockedProvider mocks={[deleteMock] as never} cache={cache}>
        <Harness />
      </MockedProvider>,
    );

    await user.click(screen.getByTestId("card-delete-c-1"));

    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });
    // Mutation has not fired yet — we are still inside the 5s window.
    expect(mutationFired).toBe(false);

    // Flip the pathname; the cleanup of the previous useEffect fires
    // flushPendingDeletes, which commits the DELETE immediately.
    mockUsePathname.mockReturnValue("/cardgroups");
    rerender(
      <MockedProvider mocks={[deleteMock] as never} cache={cache}>
        <Harness />
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(mutationFired).toBe(true);
    });
  });

  // Task 4: search debounce — only one fetch fires after 300ms.
  it("search debounces by 300ms and fires exactly one network request", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    let searchCalls = 0;
    const searchMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "hello" },
      },
      result: () => {
        searchCalls += 1;
        return { data: { cardsByCardgroupConnection: connection([CARD_1]) } };
      },
    };

    renderClient([searchMock], [CARD_1, CARD_2], { cache });

    const input = screen.getByTestId("cards-search-input");
    // Type "hello" rapidly — debounce should collapse this into one fetch.
    await user.type(input, "hello");

    // Within the debounce window, no fetch yet.
    expect(searchCalls).toBe(0);

    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      expect(searchCalls).toBe(1);
    });
  });

  // Task 4: searchQuery change resets the in-flight guard ref AND
  // fetchMoreError. We verify by triggering the IO sentinel after a search
  // change and observing that fetchMore is allowed to proceed (the guard was
  // reset). The leak spy in afterEach catches any unmatched leaked request.
  it("searchQuery change resets fetchingRef and fetchMoreError", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    // Initial query mock (cache-served, kept defensively).
    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };

    // Search "hello" → emits one card, hasNextPage=false (no fetchMore needed).
    const searchMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "hello" },
      },
      result: { data: { cardsByCardgroupConnection: connection([CARD_1]) } },
    };

    renderClient([initialMock, searchMock], [CARD_1, CARD_2], { cache });

    // Type → debounce → search mock consumed.
    const input = screen.getByTestId("cards-search-input");
    await user.type(input, "hello");
    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      expect(screen.queryByText("Bye")).not.toBeInTheDocument();
      expect(screen.getByText("Hello")).toBeInTheDocument();
    });
    // No leaks: leakSpy.assertNoLeaks() in afterEach proves the cache key
    // matched cleanly between SSR seed, useQuery, and the search refetch.
  });

  // Task 7: empty state pattern 2 — search yields zero matches, Clear search
  // resets searchInput AND searchQuery.
  it("renders no-hits state with Clear search button when search has zero matches", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    const searchMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "xyz" },
      },
      result: { data: { cardsByCardgroupConnection: connection([]) } },
    };

    renderClient([searchMock], [CARD_1, CARD_2], { cache });

    const input = screen.getByTestId("cards-search-input") as HTMLInputElement;
    await user.type(input, "xyz");
    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      expect(screen.getByTestId("cards-empty-search")).toBeInTheDocument();
    });
    expect(screen.getByText(/No cards match "xyz"/)).toBeInTheDocument();

    // Clear search resets both searchInput and searchQuery.
    await user.click(screen.getByTestId("cards-clear-search"));

    await waitFor(() => {
      expect(input.value).toBe("");
    });
    // After clearing, the original cards re-appear from the cache.
    await waitFor(() => {
      expect(screen.getByText("Hello")).toBeInTheDocument();
      expect(screen.getByText("Bye")).toBeInTheDocument();
    });
  });

  // Task 7: empty state pattern 1 — zero cards, no search, shows "Add some
  // new cards to get started." plus a brand-coloured Add card CTA.
  it("renders no-cards empty state with Add card CTA when totalCount is 0", () => {
    renderClient([], []);

    expect(screen.getByTestId("cards-empty")).toBeInTheDocument();
    expect(screen.getByText("Add some new cards to get started.")).toBeInTheDocument();
    expect(screen.getByTestId("cards-empty-add-card")).toBeInTheDocument();
  });

  // Task 5b regression guard: bulk delete still uses AlertDialog.
  it("bulk delete still requires AlertDialog confirmation (not scheduleDelete)", async () => {
    const user = userEvent.setup();

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([], [CARD_1, CARD_2], { cache });

    // Select a card so the bulk action bar is shown.
    await user.click(screen.getByTestId("card-select-c-1"));

    // The bulk action bar's Delete selected button opens an AlertDialog.
    await user.click(screen.getByTestId("cards-bulk-delete-button"));

    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
    // The dialog body is the bulk-delete confirmation copy.
    expect(screen.getByText(/Delete 1 cards\?/)).toBeInTheDocument();
  });

  // UNAUTHENTICATED on per-row delete surfaces a banner via the deleteCommitError state.
  it("UNAUTHENTICATED on delete (after timer) surfaces banner via cards-delete-error", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    const errorMock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: {
        errors: [new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } })],
      },
    };

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1]) },
    });

    renderClient([errorMock], [CARD_1], { cache });

    await user.click(screen.getByTestId("card-delete-c-1"));

    // Advance past the undo window so commitDelete fires.
    await vi.advanceTimersByTimeAsync(5000);

    await waitFor(() => {
      expect(screen.getByTestId("cards-delete-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
  });

  // useQuery error surfaces a banner via the cards-query-error testid.
  it("useQuery error renders banner with cards-query-error testid", async () => {
    const queryErrorMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: DEFAULT_VARS,
      },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };

    renderClient([queryErrorMock], [CARD_1, CARD_2], { skipSeed: true });

    const banner = await screen.findByTestId("cards-query-error");
    expect(banner).toHaveTextContent("boom");
  });

  // Sentinel + fetchMore: after the in-flight guard resets, the IO callback
  // can fire fetchMore again. This pairs with the leak spy to ensure the
  // cache key matches across fetchMore.
  it("sentinel intersection fetches next page using cardsDefaultVars-shaped vars", async () => {
    const cache = new InMemoryCache();
    const page1 = connection([CARD_1, CARD_2], true);
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    const CARD_3 = { ...CARD_1, id: "c-3", front: "Three", back: "Tres" };
    const page2Mock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      result: {
        data: { cardsByCardgroupConnection: connection([CARD_3]) },
      },
    };

    renderClient([initialMock, page2Mock], [CARD_1, CARD_2], { cache });

    expect(await screen.findByText("Hello")).toBeInTheDocument();
    fireIntersect();

    await waitFor(() => {
      expect(screen.getByText("Three")).toBeInTheDocument();
    });
  });
});
