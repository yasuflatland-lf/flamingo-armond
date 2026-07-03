// @vitest-environment happy-dom
import { fireEvent, screen } from "@testing-library/react";
import { useMemo, useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { FLAMINGO_EVENT } from "@/lib/events/flamingo-events";
import { renderWithIntl } from "@/test/render-with-intl";

// Count every CardRow render by counting its single SwipeableRow child render.
// The mock is unmemoized, so it re-renders iff its parent CardRow re-renders,
// making this counter a faithful proxy for CardRow render count. Mocking the
// gesture row also drops the @react-spring / @use-gesture dependency (canonical
// gesture coverage lives in swipeable-row.test.tsx).
let cardRowRenders = 0;
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function SwipeableRowMock(
      {
        children,
      }: {
        children: React.ReactNode;
        onDelete: () => void;
        disabled?: boolean;
        ariaLabel: string | null;
      },
      _ref: React.Ref<{ close(): void }>,
    ) {
      cardRowRenders += 1;
      return <div data-testid="swipeable-row-mock">{children}</div>;
    }),
  };
});

import { CardListScreen, type CardListScreenProps } from "./card-list-screen";

type Edge = { cursor: string; node: { id: string; front: string; back: string } };

function makeEdges(n: number): Edge[] {
  return Array.from({ length: n }, (_, i) => ({
    cursor: `cur-${i}`,
    node: { id: `c-${i}`, front: `Front ${i}`, back: `Back ${i}` },
  }));
}

/**
 * Renders the shared list screen with stable connection + mutation identities
 * (as the production hooks provide) so the only thing that changes across a
 * re-render is what the individual test drives (the search input value or the
 * bulk-selection set).
 */
function Harness({ edges }: { edges: Edge[] }) {
  const [input, setInput] = useState("");

  const connection = useMemo<CardListScreenProps["connection"]>(
    () => ({
      edges,
      pageInfo: { hasNextPage: false },
      totalCount: edges.length,
      fetchingMore: false,
      fetchMoreError: null,
      retryFetchMore: () => {},
      sentinelRef: { current: null },
      queryError: null,
    }),
    [edges],
  );

  const mutations = useMemo<CardListScreenProps["mutations"]>(
    () => ({
      createCard: async () => ({ status: "success" }),
      updateCard: async () => ({ status: "success" }),
      deleteRow: () => {},
      deleteCards: async () => undefined,
      creating: false,
      updating: false,
      bulkDeleting: false,
      createError: null,
      updateError: null,
      bulkDeleteError: null,
      deleteRowError: null,
      resetCreateCard: () => {},
    }),
    [],
  );

  const search = {
    searchOpen: false,
    input,
    query: input === "" ? null : input,
    setInput,
    clear: () => setInput(""),
    closeSearch: () => {},
  } as CardListScreenProps["search"];

  return (
    <CardListScreen
      search={search}
      connection={connection}
      mutations={mutations}
      ownerId="g-1"
      addCardEvent={{ name: FLAMINGO_EVENT.addCard, matches: () => false }}
      bulkDeleteLog={{ scope: "[test]", ownerKey: "cardgroupId" }}
      importSheetTitle="Import"
      renderImportForm={() => null}
    />
  );
}

describe("<CardListScreen> row memoization", () => {
  beforeEach(() => {
    cardRowRenders = 0;
  });

  it("does not re-render the loaded gesture rows on a search keystroke", () => {
    const edges = makeEdges(5);
    renderWithIntl(<Harness edges={edges} />);

    // One render per row on mount.
    expect(cardRowRenders).toBe(5);

    fireEvent.change(screen.getByTestId("cards-search-input"), { target: { value: "a" } });

    // The screen re-renders because `search.input` changed, but every CardRow is
    // memoized with referentially-stable props, so no row re-renders. Without the
    // memo this would be 10 (all five rows reconciled again).
    expect(cardRowRenders).toBe(5);
  });

  it("re-renders only the toggled row once selection mode is already active", () => {
    const edges = makeEdges(5);
    renderWithIntl(<Harness edges={edges} />);
    expect(cardRowRenders).toBe(5);

    // The first selection flips every row's `disabled` prop (count 0 -> 1), which
    // disables swipe list-wide, so all five rows legitimately re-render.
    fireEvent.click(screen.getByTestId("card-select-c-0"));
    expect(cardRowRenders).toBe(10);

    // The second selection keeps count > 0 (`disabled` unchanged for every row);
    // only the newly toggled row's `selected` prop changes, so exactly one row
    // re-renders. Without the memo this step would re-render all five.
    const before = cardRowRenders;
    fireEvent.click(screen.getByTestId("card-select-c-1"));
    expect(cardRowRenders).toBe(before + 1);
  });
});
