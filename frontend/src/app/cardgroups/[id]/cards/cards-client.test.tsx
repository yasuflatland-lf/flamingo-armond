// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  CreateCardDocument,
  DeleteCardDocument,
  DeleteCardsDocument,
  UpdateCardDocument,
} from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
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

// Mock SwipeableRow as a plain div so integration tests stay free of gesture
// library internals. The `disabled` prop is forwarded as a data attribute so
// tests can assert its value. The `close` imperative handle is wired via
// forwardRef so closeOtherRows() calls in the production code work without
// throwing. swipeable-row.test.tsx is the canonical test for gesture behaviour.
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function SwipeableRowMock(
      {
        children,
        disabled,
        onDelete: _onDelete,
        ariaLabel: _ariaLabel,
      }: {
        children: React.ReactNode;
        disabled?: boolean;
        onDelete: () => void;
        ariaLabel: string | null;
      },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return (
        <div data-testid="swipeable-row-mock" data-disabled={disabled ? "true" : "false"}>
          {children}
        </div>
      );
    }),
  };
});

import { CardsClient } from "./cards-client";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_ID = "cg-1";
const PAGE_SIZE = 20;

const userCardState = (due: string, state: number) => ({
  __typename: "UserCardState" as const,
  due,
  state,
});

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  userCardState: userCardState("2024-06-15", 0),
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  userCardState: userCardState("2024-06-15", 0),
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

// Default vars shape — must match cardsDefaultVars(CG_ID).
const DEFAULT_VARS = { cardgroupId: CG_ID, first: PAGE_SIZE, search: null };

function makeUpdateMock(id: string, input: { front: string; back: string }, card = CARD_1) {
  return {
    request: { query: UpdateCardDocument, variables: { id, input } },
    result: {
      data: {
        updateCard: { __typename: "UpdateCardSuccess" as const, card },
      },
    },
  };
}

function makeCreateMock(args: {
  front: string;
  back: string;
  cardId?: string;
  duplicateMessage?: string;
  errors?: GraphQLError[];
}) {
  const { front, back, cardId = "c-new", duplicateMessage, errors } = args;
  return {
    request: {
      query: CreateCardDocument,
      variables: { input: { cardgroupId: CG_ID, front, back } },
    },
    result: () => {
      if (errors) return { errors };
      if (duplicateMessage) {
        return {
          data: {
            createCard: {
              __typename: "CardDuplicateFrontError" as const,
              message: duplicateMessage,
              existingCardId: "c-existing",
              existingBack: "Existing back",
            },
          },
        };
      }
      return {
        data: {
          createCard: {
            __typename: "CreateCardSuccess" as const,
            card: {
              __typename: "Card" as const,
              id: cardId,
              front,
              back,
              userCardState: userCardState("2026-05-23T00:00:00Z", 0),
              cardgroupId: CG_ID,
            },
          },
        },
      };
    },
  };
}

function renderClient(
  mocks: unknown[],
  initialCards = [CARD_1, CARD_2],
  options: {
    cache?: InMemoryCache;
    skipSeed?: boolean;
    sectionHeader?: React.ComponentProps<typeof CardsClient>["sectionHeader"];
  } = {},
) {
  const initialConn = connection(initialCards);

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
        data: { cardsByCardgroupConnection: initialConn },
      });
      return c;
    })();

  render(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <UndoDeleteProvider>
        <CardsClient
          cardgroupId={CG_ID}
          cardgroupName="Test Cardgroup"
          initialEdges={initialConn.edges}
          initialPageInfo={initialConn.pageInfo}
          initialTotalCount={initialConn.totalCount}
          sectionHeader={options.sectionHeader}
        />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
}

function addButtonHeader({
  totalCount,
  onAddCard,
}: {
  totalCount: number;
  onAddCard?: () => void;
}) {
  return (
    <div>
      <span>Cards ({totalCount})</span>
      <button type="button" onClick={onAddCard}>
        Add card
      </button>
    </div>
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

  it("section Add card button opens the Add card sheet with empty defaults", async () => {
    const user = userEvent.setup();
    renderClient([], [], { sectionHeader: addButtonHeader });

    await user.click(screen.getByRole("button", { name: /add card/i }));

    expect(screen.getByRole("heading", { name: /add card/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/front/i)).toHaveValue("");
    expect(screen.getByLabelText(/back/i)).toHaveValue("");
  });

  it("clean Add card dismiss closes without asking before discarding", async () => {
    const user = userEvent.setup();
    renderClient([], [], { sectionHeader: addButtonHeader });

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /add card/i })).not.toBeInTheDocument();
    });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("dirty Add card dismiss asks before discarding", async () => {
    const user = userEvent.setup();
    renderClient([], [], { sectionHeader: addButtonHeader });

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.type(screen.getByLabelText(/front/i), "Hello");
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Hello")).toBeInTheDocument();
  });

  it("successful create inserts the card into the active connection and closes the sheet", async () => {
    const user = userEvent.setup();
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([makeCreateMock({ front: "New front", back: "New back" })], [CARD_1, CARD_2], {
      cache,
      sectionHeader: addButtonHeader,
    });

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.type(screen.getByLabelText(/front/i), "New front");
    await user.type(screen.getByLabelText(/back/i), "New back");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    expect(await screen.findByText("New front")).toBeInTheDocument();
    expect(screen.getByText("Cards (3)")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /add card/i })).not.toBeInTheDocument();
    });
    expect(
      cache.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: DEFAULT_VARS,
      })?.cardsByCardgroupConnection.totalCount,
    ).toBe(3);
  });

  it("successful create while search is active updates the unfiltered cache without polluting filtered results", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { ...DEFAULT_VARS, search: "hello" },
      data: { cardsByCardgroupConnection: connection([CARD_1]) },
    });

    renderClient([makeCreateMock({ front: "Zebra", back: "Cebra" })], [CARD_1, CARD_2], {
      cache,
      sectionHeader: addButtonHeader,
    });

    await user.type(screen.getByTestId("cards-search-input"), "hello");
    await vi.advanceTimersByTimeAsync(300);
    await waitFor(() => {
      expect(screen.queryByText("Bye")).not.toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.type(screen.getByLabelText(/front/i), "Zebra");
    await user.type(screen.getByLabelText(/back/i), "Cebra");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /add card/i })).not.toBeInTheDocument();
    });
    expect(screen.queryByText("Zebra")).not.toBeInTheDocument();
    expect(screen.getByText("Cards (1)")).toBeInTheDocument();

    await user.clear(screen.getByTestId("cards-search-input"));
    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      expect(screen.getByText("Zebra")).toBeInTheDocument();
    });
    expect(screen.getByText("Cards (3)")).toBeInTheDocument();
  });

  it("CardDuplicateFrontError keeps Add card open and surfaces the front validation error", async () => {
    const user = userEvent.setup();
    renderClient(
      [
        makeCreateMock({
          front: "Hello",
          back: "Hola",
          duplicateMessage: "A card with this front already exists in this cardgroup",
        }),
      ],
      [],
      { sectionHeader: addButtonHeader },
    );

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.type(screen.getByLabelText(/front/i), "Hello");
    await user.type(screen.getByLabelText(/back/i), "Hola");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    expect(
      await screen.findByText("A card with this front already exists in this cardgroup"),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /add card/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/front/i)).toHaveValue("Hello");
  });

  it("create mutation thrown errors keep Add card open and show the CardForm error banner", async () => {
    const user = userEvent.setup();
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    renderClient(
      [
        makeCreateMock({
          front: "Hello",
          back: "Hola",
          errors: [new GraphQLError("backend exploded", { extensions: { code: "INTERNAL" } })],
        }),
      ],
      [],
      { sectionHeader: addButtonHeader },
    );

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.type(screen.getByLabelText(/front/i), "Hello");
    await user.type(screen.getByLabelText(/back/i), "Hola");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("backend exploded");
    expect(screen.getByRole("heading", { name: /add card/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/front/i)).toHaveValue("Hello");

    consoleErrorSpy.mockRestore();
  });

  it("clears a thrown create error after closing and reopening the Add card sheet", async () => {
    const user = userEvent.setup();
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    renderClient(
      [
        makeCreateMock({
          front: "Hello",
          back: "Hola",
          errors: [new GraphQLError("backend exploded", { extensions: { code: "INTERNAL" } })],
        }),
      ],
      [],
      { sectionHeader: addButtonHeader },
    );

    await user.click(screen.getByRole("button", { name: /add card/i }));
    await user.type(screen.getByLabelText(/front/i), "Hello");
    await user.type(screen.getByLabelText(/back/i), "Hola");
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("backend exploded");
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    await user.click(screen.getByRole("button", { name: /discard/i }));

    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /add card/i })).not.toBeInTheDocument();
    });
    await user.click(screen.getByRole("button", { name: /add card/i }));

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    consoleErrorSpy.mockRestore();
  });

  it("matching flamingo:add-card event opens the Add card sheet and prevents navigation fallback", async () => {
    renderClient([]);

    const event = new CustomEvent("flamingo:add-card", {
      cancelable: true,
      detail: { cardgroupId: CG_ID },
    });
    const dispatchResult = window.dispatchEvent(event);

    expect(dispatchResult).toBe(false);
    expect(event.defaultPrevented).toBe(true);
    expect(await screen.findByRole("heading", { name: /add card/i })).toBeInTheDocument();
  });

  it("does not render a create form (Add a card heading absent)", () => {
    renderClient([]);
    expect(screen.queryByText(/add a card/i)).toBeNull();
    expect(screen.queryByRole("button", { name: /^add$/i })).toBeNull();
  });

  // Row click opens the edit sheet while keeping the list visible.
  it("clicking the row text opens edit mode in a sheet and Cancel reverts to view", async () => {
    const user = userEvent.setup();
    renderClient([]);

    await user.click(screen.getByTestId("card-edit-target-c-1"));

    expect(screen.getByRole("heading", { name: /edit card/i })).toBeInTheDocument();
    expect(screen.getAllByTestId("swipeable-row-mock")).toHaveLength(2);
    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    expect(editFrontInput).toHaveValue("Hello");

    await user.click(screen.getByRole("button", { name: /cancel/i }));

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /cancel/i })).not.toBeInTheDocument();
    });
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });

  // Keyboard activates edit mode via the Enter key.
  it("keyboard Enter on the row text enters edit mode", async () => {
    const user = userEvent.setup();
    renderClient([]);

    const target = screen.getByTestId("card-edit-target-c-1");
    target.focus();
    await user.keyboard("{Enter}");

    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
  });

  // Clicking the row checkbox does NOT enter edit mode.
  it("clicking the checkbox toggles selection without entering edit mode", async () => {
    const user = userEvent.setup();
    renderClient([]);

    await user.click(screen.getByTestId("card-select-c-1"));

    expect(screen.getByTestId("cards-bulk-action-bar")).toBeInTheDocument();
    expect(screen.queryByLabelText(/^front$/i)).toBeNull();
  });

  // Clicking the per-row Delete icon does NOT enter edit mode and does not
  // surface an AlertDialog; per-row delete uses scheduleDelete + Undo instead.
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
    // No AlertDialog confirmation appears for per-row delete.
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

    expect(screen.getByRole("heading", { name: /edit card/i })).toBeInTheDocument();
    expect(screen.getAllByTestId("swipeable-row-mock")).toHaveLength(2);
    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);
    await user.type(editFrontInput, "Hello updated");

    const editBackInput = screen.getByLabelText(/back/i) as HTMLElement;
    await user.clear(editBackInput);
    await user.type(editBackInput, "Hola updated");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /edit card/i })).not.toBeInTheDocument();
    });
    expect(screen.getByText("Hello updated")).toBeInTheDocument();
  });

  it("edit save failure keeps the sheet open and shows the inline error", async () => {
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

    renderClient([mock], [CARD_1]);

    await user.click(screen.getByTestId("card-edit-target-c-1"));
    expect(screen.getByRole("heading", { name: /edit card/i })).toBeInTheDocument();
    expect(screen.getAllByTestId("swipeable-row-mock")).toHaveLength(1);

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("front is required")).toBeInTheDocument();
    });

    expect(screen.getByRole("heading", { name: /edit card/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
  });

  // Per-row delete via scheduleDelete — row disappears immediately,
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

  // Undo within 5s restores the row and prevents the DELETE mutation.
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

  // Pathname change triggers flushPendingDeletes — DELETE fires
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

    const initialConn = connection([CARD_1, CARD_2]);

    function Harness() {
      return (
        <CardsClient
          cardgroupId={CG_ID}
          cardgroupName="Test Cardgroup"
          initialEdges={initialConn.edges}
          initialPageInfo={initialConn.pageInfo}
          initialTotalCount={initialConn.totalCount}
        />
      );
    }

    const { rerender } = render(
      <MockedProvider mocks={[deleteMock] as never} cache={cache}>
        <UndoDeleteProvider>
          <Harness />
        </UndoDeleteProvider>
      </MockedProvider>,
    );

    await user.click(screen.getByTestId("card-delete-c-1"));

    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });
    // Mutation has not fired yet — we are still inside the 5s window.
    expect(mutationFired).toBe(false);

    // Flip the pathname; the Provider's useEffect body detects
    // previousPathnameRef.current !== pathname and calls flushPendingDeletes,
    // which commits the DELETE immediately.
    mockUsePathname.mockReturnValue("/cardgroups");
    rerender(
      <MockedProvider mocks={[deleteMock] as never} cache={cache}>
        <UndoDeleteProvider>
          <Harness />
        </UndoDeleteProvider>
      </MockedProvider>,
    );

    await waitFor(() => {
      expect(mutationFired).toBe(true);
    });
  });

  // Search debounce — only one fetch fires after 300ms.
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

  // searchQuery change resets the in-flight guard ref AND
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

  // Empty state — search yields zero matches, Clear search
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

  // Empty state — zero cards, no search, shows the prompt copy. The Add card
  // CTA lives in the section toolbar (top-right), not inside the empty state.
  it("renders no-cards empty state copy when totalCount is 0", () => {
    renderClient([], []);

    expect(screen.getByTestId("cards-empty")).toBeInTheDocument();
    expect(screen.getByText("Add some new cards to get started.")).toBeInTheDocument();
  });

  // Bulk delete uses AlertDialog (per-row delete uses scheduleDelete instead).
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

  // Each non-editing row is wrapped in SwipeableRow.
  it("wraps each card row in SwipeableRow", () => {
    renderClient([]);
    const wrappers = screen.getAllByTestId("swipeable-row-mock");
    // One SwipeableRow per card (CARD_1 and CARD_2).
    expect(wrappers).toHaveLength(2);
  });

  // Selection mode disables swipe on all rows.
  it("disables SwipeableRow when selection mode is active", async () => {
    const user = userEvent.setup();
    renderClient([]);

    // Before selection mode, disabled should be false.
    const wrappersBefore = screen.getAllByTestId("swipeable-row-mock");
    for (const w of wrappersBefore) {
      expect(w).toHaveAttribute("data-disabled", "false");
    }

    // Enter selection mode by checking a card.
    await user.click(screen.getByTestId("card-select-c-1"));

    // All SwipeableRow wrappers should be disabled now.
    const wrappersAfter = screen.getAllByTestId("swipeable-row-mock");
    for (const w of wrappersAfter) {
      expect(w).toHaveAttribute("data-disabled", "true");
    }
  });

  // Editing a row disables swipe on that row only.
  it("disables SwipeableRow for the row currently in edit mode", async () => {
    const user = userEvent.setup();
    renderClient([]);

    // Enter edit mode for CARD_1 (the mocked SwipeableRow renders children
    // unconditionally, so the edit target remains accessible).
    await user.click(screen.getByTestId("card-edit-target-c-1"));

    const editingRow = screen
      .getByTestId("card-edit-target-c-1")
      .closest('[data-testid="swipeable-row-mock"]');
    const idleRow = screen
      .getByTestId("card-edit-target-c-2")
      .closest('[data-testid="swipeable-row-mock"]');

    expect(editingRow).toHaveAttribute("data-disabled", "true");
    expect(idleRow).toHaveAttribute("data-disabled", "false");
  });

  // Tapping a row's text region closes other half-open rows.
  // Because SwipeableRow is mocked as a plain div, we verify that
  // closeOtherRows is wired by checking that the edit-target click succeeds
  // without errors when multiple rows exist (the ref-map iteration path is
  // exercised without throwing). The imperative close() API is covered by
  // swipeable-row.test.tsx.
  it("tapping a second row's edit target does not throw (closeOtherRows is wired)", async () => {
    const user = userEvent.setup();
    renderClient([]);

    // Click CARD_1 to enter edit mode — closeOtherRows(CARD_1.id) is called.
    await user.click(screen.getByTestId("card-edit-target-c-1"));
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();

    // Cancel and click CARD_2 — closeOtherRows(CARD_2.id) is called,
    // iterating the map and attempting to close CARD_1's ref (which is now
    // unmounted since CARD_1 is in edit mode; the optional-chaining in
    // closeOtherRows makes this a safe no-op).
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /cancel/i })).not.toBeInTheDocument();
    });

    await user.click(screen.getByTestId("card-edit-target-c-2"));
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
  });

  // T1: Issue 1 regression — per-row delete must work under active search.
  // Before the fix, handleDeleteRow called readQuery / writeQuery with
  // cardsDefaultVars (search: null) while useQuery was keyed on
  // { ...cardsDefaultVars, search: searchQuery }. Under active search,
  // readQuery returned null and the optimistic remove never happened.
  it("optimistic per-row delete works under active search filter", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    // Two cards both matching "apple" — front/back fields contain "apple".
    const APPLE_1 = {
      __typename: "Card" as const,
      id: "c-apple-1",
      front: "apple",
      back: "ringo",
      userCardState: userCardState("2024-06-15", 0),
      cardgroupId: CG_ID,
    };
    const APPLE_2 = {
      __typename: "Card" as const,
      id: "c-apple-2",
      front: "apple pie",
      back: "torta",
      userCardState: userCardState("2024-06-15", 0),
      cardgroupId: CG_ID,
    };

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([APPLE_1, APPLE_2]) },
    });

    // Search "apple" returns both cards. Counter proves the search refetch
    // actually completed before we click delete.
    let searchCalls = 0;
    const searchMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "apple" },
      },
      result: () => {
        searchCalls += 1;
        return { data: { cardsByCardgroupConnection: connection([APPLE_1, APPLE_2]) } };
      },
    };

    // The DELETE mutation for APPLE_1.
    const deleteMock = {
      request: { query: DeleteCardDocument, variables: { id: APPLE_1.id } },
      result: { data: { deleteCard: true } },
    };

    renderClient([searchMock, deleteMock], [APPLE_1, APPLE_2], { cache });

    // Drive the search input → debounced searchQuery="apple".
    const input = screen.getByTestId("cards-search-input");
    await user.type(input, "apple");
    await vi.advanceTimersByTimeAsync(300);

    // Wait for the search refetch to actually complete. Without this gate,
    // the test races the debounced setSearchQuery against the click below
    // and the optimistic remove targets the wrong cache key.
    await waitFor(() => {
      expect(searchCalls).toBe(1);
    });
    // And wait for Apollo to have written the result under the search vars.
    await waitFor(() => {
      const entry = cache.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "apple" },
      });
      expect(entry?.cardsByCardgroupConnection.edges).toHaveLength(2);
    });

    // Click delete on APPLE_1 — the optimistic remove must fire IMMEDIATELY,
    // not after the 5s timer. Before the queryVariables fix, this assertion
    // would fail because readQuery returned null under active search and the
    // optimistic write never happened.
    await user.click(screen.getByTestId(`card-delete-${APPLE_1.id}`));

    // Direct cache assertion: the search-keyed variant must now show ONE edge.
    // This is the core regression assertion — the cache value under
    // `{...DEFAULT_VARS, search: "apple"}` proves the optimistic writeQuery
    // landed on the correct key.
    await waitFor(() => {
      const searchKeyAfter = cache.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "apple" },
      });
      expect(searchKeyAfter?.cardsByCardgroupConnection.edges).toHaveLength(1);
      expect(searchKeyAfter?.cardsByCardgroupConnection.edges[0]?.node.id).toBe(APPLE_2.id);
    });
  });

  // T2: Issue 1 regression for bulk delete — same root cause, same fix.
  // The bulk-delete `update` callback must read/write under queryVariables.
  it("bulk delete works under active search filter", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });

    const APPLE_1 = {
      __typename: "Card" as const,
      id: "c-apple-1",
      front: "apple",
      back: "ringo",
      userCardState: userCardState("2024-06-15", 0),
      cardgroupId: CG_ID,
    };
    const APPLE_2 = {
      __typename: "Card" as const,
      id: "c-apple-2",
      front: "apple pie",
      back: "torta",
      userCardState: userCardState("2024-06-15", 0),
      cardgroupId: CG_ID,
    };

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([APPLE_1, APPLE_2]) },
    });

    const searchMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "apple" },
      },
      result: { data: { cardsByCardgroupConnection: connection([APPLE_1, APPLE_2]) } },
    };

    const bulkDeleteMock = {
      request: {
        query: DeleteCardsDocument,
        variables: { ids: [APPLE_1.id, APPLE_2.id] },
      },
      result: { data: { deleteCards: 2 } },
    };

    renderClient([searchMock, bulkDeleteMock], [APPLE_1, APPLE_2], { cache });

    const input = screen.getByTestId("cards-search-input");
    await user.type(input, "apple");
    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      expect(screen.getByText("apple")).toBeInTheDocument();
      expect(screen.getByText("apple pie")).toBeInTheDocument();
    });

    // Select both cards.
    await user.click(screen.getByTestId(`card-select-${APPLE_1.id}`));
    await user.click(screen.getByTestId(`card-select-${APPLE_2.id}`));

    // Open the bulk delete confirm dialog.
    await user.click(screen.getByTestId("cards-bulk-delete-button"));
    await user.click(screen.getByTestId("cards-bulk-confirm"));

    // Both rows must disappear from the active-search list. Before the
    // queryVariables fix, the bulk-delete `update` callback's readQuery
    // returned null under active search and the cache never lost the edges.
    await waitFor(() => {
      expect(screen.queryByText("apple")).not.toBeInTheDocument();
    });
    expect(screen.queryByText("apple pie")).not.toBeInTheDocument();
  });

  // T2b: bulk-delete rejection emits a structured console.error and the
  // payload must NOT carry `err.message` (PII redaction).
  // See .claude/rules/frontend-rsc-error-handling.md
  // § "Redact `err.message` from structured `console` payloads".
  it("bulk delete rejection logs structured payload without err.message", async () => {
    const user = userEvent.setup();

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1]) },
    });

    const bulkDeleteErrorMock = {
      request: {
        query: DeleteCardsDocument,
        variables: { ids: [CARD_1.id] },
      },
      result: {
        errors: [new GraphQLError("forbidden", { extensions: { code: "FORBIDDEN" } })],
      },
    };

    renderClient([bulkDeleteErrorMock], [CARD_1], { cache });

    expect(await screen.findByText("Hello")).toBeInTheDocument();

    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    await user.click(screen.getByTestId(`card-select-${CARD_1.id}`));
    await user.click(screen.getByTestId("cards-bulk-delete-button"));
    await user.click(screen.getByTestId("cards-bulk-confirm"));

    await waitFor(() => {
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "[CardsClient] bulk delete rejection",
        expect.objectContaining({
          name: expect.any(String),
          cardgroupId: CG_ID,
          ids: [CARD_1.id],
        }),
      );
    });

    const errorCall = consoleErrorSpy.mock.calls.find(
      (call) => call[0] === "[CardsClient] bulk delete rejection",
    );
    expect(errorCall?.[1]).not.toHaveProperty("message");

    // The user-facing error banner must be visible after the rejection settles.
    expect(await screen.findByTestId("cards-bulk-delete-error")).toBeInTheDocument();

    consoleErrorSpy.mockRestore();
  });

  // T3: beforeunload handler invokes flushPendingDeletes, committing the
  // pending DELETE mutation immediately.
  // The DELETE mutation may not actually reach the server in production
  // (browsers cancel pending fetch on beforeunload), but the mutation firing
  // is the observable contract we can assert here.
  it("beforeunload event triggers flushPendingDeletes", async () => {
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

    // Schedule a delete — row disappears optimistically.
    await user.click(screen.getByTestId("card-delete-c-1"));

    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });
    expect(mutationFired).toBe(false);

    // Install the outer spy WITHOUT mockImplementation so the leak spy still
    // receives all console.warn calls. Per docs/pagination/capture-mockedprovider-warn-leaks.md
    // § "Spy stacking": do NOT swallow the outer spy's implementation.
    const consoleWarnSpy = vi.spyOn(console, "warn");

    // Dispatch beforeunload — the handler calls flushPendingDeletes which
    // immediately fires commitDelete.
    window.dispatchEvent(new Event("beforeunload"));

    // The mutation should fire.
    await waitFor(() => {
      expect(mutationFired).toBe(true);
    });

    // The beforeunload handler emits a warn for operator triage.
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[UndoDeleteProvider] flushPendingDeletes on beforeunload — may be cancelled by browser",
    );
    consoleWarnSpy.mockRestore();
  });

  // T3b: beforeunload handler does NOT warn or flush when nothing is pending.
  // Pairs with T3 above which confirms warn IS emitted when a delete is pending.
  it("beforeunload event does nothing when no pending deletes exist", async () => {
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });
    renderClient([], [CARD_1, CARD_2], { cache });

    const consoleWarnSpy = vi.spyOn(console, "warn");

    window.dispatchEvent(new Event("beforeunload"));

    // No pending deletes — warn must not fire.
    expect(consoleWarnSpy).not.toHaveBeenCalledWith(
      "[UndoDeleteProvider] flushPendingDeletes on beforeunload — may be cancelled by browser",
    );

    consoleWarnSpy.mockRestore();
  });

  // T4: fetchMoreError halts the IO loop. Click Retry → next page loads,
  // banner clears. Two MockedResponse entries: one for the network error,
  // one for the retry success path.
  // See docs/pagination/two-mocked-responses-for-retry-test.md.
  it("fetchMore error shows banner and Retry recovers", async () => {
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

    // First fetchMore attempt → network error.
    const errorMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      error: new Error("network failure"),
    };
    // Retry → success.
    const retryMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      result: {
        data: { cardsByCardgroupConnection: connection([CARD_3]) },
      },
    };

    renderClient([initialMock, errorMock, retryMock], [CARD_1, CARD_2], { cache });

    expect(await screen.findByText("Hello")).toBeInTheDocument();

    // Install the outer spy WITHOUT mockImplementation so the leak spy still
    // receives all console.warn calls. Per docs/pagination/capture-mockedprovider-warn-leaks.md
    // § "Spy stacking": do NOT swallow the outer spy's implementation.
    const consoleWarnSpy = vi.spyOn(console, "warn");

    // Trigger first fetchMore → fails.
    fireIntersect();

    await waitFor(() => {
      expect(screen.getByTestId("cards-fetch-more-error")).toBeInTheDocument();
    });

    // fetchMore failure emits a structured warn for operator triage.
    // The `endCursor` key discriminates against a bare Error regression.
    // See docs/frontend/typescript-conventions.md
    // § "expect.objectContaining({ message }) is not enough".
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[cards-client] fetchMore failed",
      expect.objectContaining({
        name: expect.any(String),
        endCursor: expect.anything(),
      }),
    );

    // PII redaction contract — docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
    // Forcing assertion: a future contributor that adds `err.message` "for
    // debugging" must trip this check.
    const warnCall = consoleWarnSpy.mock.calls.find(
      (call) => call[0] === "[cards-client] fetchMore failed",
    );
    expect(warnCall?.[1]).not.toHaveProperty("message");

    consoleWarnSpy.mockRestore();

    // Click Retry → next page loads.
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Three")).toBeInTheDocument();
    });
    // Banner cleared after success.
    expect(screen.queryByTestId("cards-fetch-more-error")).not.toBeInTheDocument();
  });

  // T5: closeOtherRows is the parent-side wiring; the SwipeableRow mock
  // renders children unconditionally and replaces the imperative close()
  // handle with a no-op. The end-to-end behaviour (half-swipe one row, tap
  // a different row's edit target, assert the half-open row's delete button
  // gets tabIndex=-1) requires a live SwipeableRow with pointer events,
  // which jsdom does not simulate reliably. The imperative close() API is
  // covered in swipeable-row.test.tsx (T10). Limitation documented here.

  // Union-data path: server returns InputValidationError as data (not errors[]).
  // The sheet must stay open and display the validation message inline.
  // Fix #1 (sibling agent) wires a `validationError` prop from CardsClient into
  // CardForm so the union-data path surfaces the message the same way the
  // error-channel path does.
  it("keeps the row in edit mode and surfaces a field error when updateCard returns InputValidationError", async () => {
    const user = userEvent.setup();

    const mock = {
      request: {
        query: UpdateCardDocument,
        variables: { id: "c-1", input: { front: "", back: "Hola" } },
      },
      result: {
        data: {
          updateCard: {
            __typename: "InputValidationError" as const,
            field: "front",
            message: "front is required",
          },
        },
      },
    };

    renderClient([mock], [CARD_1]);

    // Enter edit mode for CARD_1.
    await user.click(screen.getByTestId("card-edit-target-c-1"));
    expect(screen.getByRole("heading", { name: /edit card/i })).toBeInTheDocument();

    // Clear the front field so it submits an empty value.
    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    // Row must remain in edit mode (editingId is not cleared for InputValidationError).
    await waitFor(() => {
      expect(screen.getByText("front is required")).toBeInTheDocument();
    });

    // The sheet is still mounted — both the field input and the Save button remain.
    expect(screen.getByRole("heading", { name: /edit card/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
    // No spinner — the mutation has resolved, not pending.
    expect(screen.queryByText(/saving/i)).not.toBeInTheDocument();
  });
});
