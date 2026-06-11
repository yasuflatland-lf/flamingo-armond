// @vitest-environment jsdom
/**
 * Tests for <CardgroupPickerSheet>.
 *
 * The component renders a Radix Sheet (portal-based) that queries
 * MyCardgroupsConnectionQuery via Apollo useQuery, skipping the query while
 * closed. Tests use MockedProvider from @apollo/client/testing.
 *
 * Leak-spy note: MyCardgroupsConnection is a paginated Connection, so the
 * leak-guard rule in docs/pagination/capture-mockedprovider-warn-leaks.md
 * applies. installApolloMockLeakSpy is used for all tests with a
 * MockedProvider.
 */
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import enMessages from "../../../messages/en.json";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../__tests__/utils/mock-apollo-paginated";
import CardgroupPickerSheet from "./cardgroup-picker-sheet";

// ---------------------------------------------------------------------------
// Module mocks
// ---------------------------------------------------------------------------

vi.mock("next/link", () => ({
  default: ({ children, ...rest }: { children: React.ReactNode; [key: string]: unknown }) => (
    <a {...rest}>{children}</a>
  ),
}));

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_1 = {
  __typename: "Cardgroup" as const,
  id: "cg-1",
  name: "Spanish Vocab",
  updatedAt: "2026-01-01T00:00:00Z",
};
const CG_2 = {
  __typename: "Cardgroup" as const,
  id: "cg-2",
  name: "Japanese Kanji",
  updatedAt: "2026-01-02T00:00:00Z",
};
const CG_3 = {
  __typename: "Cardgroup" as const,
  id: "cg-3",
  name: "French Phrases",
  updatedAt: "2026-01-03T00:00:00Z",
};

function makeConnectionData(cardgroups: (typeof CG_1)[]) {
  return {
    myCardgroupsConnection: {
      __typename: "CardgroupConnection" as const,
      edges: cardgroups.map((cg) => ({
        __typename: "CardgroupEdge" as const,
        cursor: cg.id,
        node: cg,
      })),
      pageInfo: {
        __typename: "PageInfo" as const,
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: cardgroups[0]?.id ?? null,
        endCursor: cardgroups[cardgroups.length - 1]?.id ?? null,
      },
      totalCount: cardgroups.length,
    },
  };
}

function baseMocks(cardgroups: (typeof CG_1)[]): MockedResponse[] {
  return [
    {
      request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
      result: { data: makeConnectionData(cardgroups) },
    },
  ];
}

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

type SheetProps = {
  open?: boolean;
  selectedId?: string | null;
  mocks?: MockedResponse[];
  createReturnTo?: string;
};

function renderSheet({
  open = true,
  selectedId = null,
  mocks = baseMocks([CG_1, CG_2]),
  createReturnTo = "/cards/new",
}: SheetProps = {}) {
  const onOpenChange = vi.fn();
  const onSelect = vi.fn();

  render(
    <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
      <MockedProvider mocks={mocks}>
        <CardgroupPickerSheet
          open={open}
          onOpenChange={onOpenChange}
          selectedId={selectedId}
          onSelect={onSelect}
          createReturnTo={createReturnTo}
        />
      </MockedProvider>
    </NextIntlClientProvider>,
  );

  return { onOpenChange, onSelect };
}

// ---------------------------------------------------------------------------
// Leak guard — installed per-test in beforeEach, asserted in afterEach
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({ operationNames: ["MyCardgroupsConnection"] });
});

afterEach(() => {
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("<CardgroupPickerSheet>", () => {
  // S1: closed sheet skips the query entirely — no mock entries consumed.
  it("does not fire the query when open=false", () => {
    renderSheet({ open: false, mocks: [] });
    // No mocked responses means any accidental query would trigger a leak warning
    // that our afterEach spy would catch. No assertions needed beyond a clean teardown.
    expect(screen.queryByText("Select cardgroup")).toBeNull();
  });

  // S2: loading indicator visible while the mocked response is pending.
  it("shows a loading indicator while the query is in flight", async () => {
    renderSheet({
      mocks: [
        {
          request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
          delay: Infinity,
          result: { data: makeConnectionData([]) },
        },
      ],
    });

    // Radix Sheet renders the portal content when open=true.
    // While the query is pending (delay: Infinity), Apollo sets loading=true.
    const loading = await screen.findByText(/loading/i);
    expect(loading).toBeInTheDocument();
  });

  // S3: error state shows a message and a Retry button.
  it("shows error message and Retry button when the query fails", async () => {
    const errorMock: MockedResponse = {
      request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
      error: new Error("network failure"),
    };

    renderSheet({ mocks: [errorMock] });

    expect(await screen.findByText(/failed to load cardgroups/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  // S12: clicking Retry triggers a refetch — error UI replaced by loaded list.
  it("clicking Retry triggers a refetch and shows the cardgroup list on success", async () => {
    const user = userEvent.setup();

    // Two mock entries for the same query: first errors, second succeeds.
    // MockedProvider consumes entries in order; the refetch consumes the second.
    const retryMocks: MockedResponse[] = [
      {
        request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
        error: new Error("network failure"),
      },
      {
        request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
        result: { data: makeConnectionData([CG_1, CG_2]) },
      },
    ];

    renderSheet({ mocks: retryMocks });

    // Wait for the error state to appear.
    expect(await screen.findByText(/failed to load cardgroups/i)).toBeInTheDocument();

    const retryButton = screen.getByRole("button", { name: /retry/i });

    // Click Retry — this fires refetch(), consuming the second mock.
    await user.click(retryButton);

    // After the refetch resolves, the cardgroup list must be rendered.
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Japanese Kanji")).toBeInTheDocument();
  });

  // S13: Retry that fails again warns once — rejection is caught and logged.
  it("S13: Retry that fails again warns once", async () => {
    const user = userEvent.setup();

    // Two mock entries that both error: initial load fails, refetch also fails.
    const doubleErrorMocks: MockedResponse[] = [
      {
        request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
        error: new Error("network failure"),
      },
      {
        request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
        error: new Error("still down"),
      },
    ];

    // Outer spy layered AFTER the leak spy (LIFO restore: outer first, then leak).
    // No mockImplementation — calls flow through to the leak spy.
    const consoleWarnSpy = vi.spyOn(console, "warn");

    try {
      renderSheet({ mocks: doubleErrorMocks });

      // Wait for the initial error state.
      expect(await screen.findByText(/failed to load cardgroups/i)).toBeInTheDocument();

      // Click Retry — refetch fires and also errors.
      await user.click(screen.getByRole("button", { name: /retry/i }));

      // The .catch handler must have logged a warn with our scope tag.
      await waitFor(() => {
        expect(consoleWarnSpy).toHaveBeenCalledWith(
          "[cardgroup-picker-sheet] refetch failed",
          expect.objectContaining({
            message: expect.any(String),
          }),
        );
      });

      // Error UI is still visible after the second failure.
      expect(screen.getByText(/failed to load cardgroups/i)).toBeInTheDocument();
    } finally {
      // Restore outer spy first (LIFO), then leak spy teardown happens in afterEach.
      consoleWarnSpy.mockRestore();
    }
  });

  // S4: success state renders the list of cardgroup names.
  it("renders all cardgroup names on successful load", async () => {
    renderSheet({ mocks: baseMocks([CG_1, CG_2, CG_3]) });

    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Japanese Kanji")).toBeInTheDocument();
    expect(screen.getByText("French Phrases")).toBeInTheDocument();
  });

  // S5: the row matching selectedId shows the Check icon (aria-pressed=true).
  it("marks the selected cardgroup row as pressed", async () => {
    renderSheet({ mocks: baseMocks([CG_1, CG_2]), selectedId: "cg-1" });

    await screen.findByText("Spanish Vocab");

    const selectedBtn = screen.getByRole("button", { name: /spanish vocab/i });
    expect(selectedBtn).toHaveAttribute("aria-pressed", "true");

    const unselectedBtn = screen.getByRole("button", { name: /japanese kanji/i });
    expect(unselectedBtn).toHaveAttribute("aria-pressed", "false");
  });

  // S6: clicking a non-selected row calls onSelect with the cardgroup id and
  //     triggers onOpenChange(false).
  it("calls onSelect and closes the sheet when a row is clicked", async () => {
    const user = userEvent.setup();
    const { onSelect, onOpenChange } = renderSheet({
      mocks: baseMocks([CG_1, CG_2]),
      selectedId: "cg-1",
    });

    await screen.findByText("Japanese Kanji");

    await user.click(screen.getByRole("button", { name: /japanese kanji/i }));

    expect(onSelect).toHaveBeenCalledWith("cg-2");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  // S7: empty state — no cardgroups shows friendly text; the standalone CTA is gone.
  it("shows empty-state copy and the inline create link when the connection is empty", async () => {
    renderSheet({ mocks: baseMocks([]) });

    expect(await screen.findByText(/don't have any cardgroups yet/i)).toBeInTheDocument();

    // The inline "Create new cardgroup…" link must also be present.
    expect(screen.getByRole("link", { name: /create new cardgroup/i })).toBeInTheDocument();
  });

  // S8: sheet title is rendered (accessibility sanity check for aria-labelledby).
  it("renders the 'Select cardgroup' sheet title when open", async () => {
    renderSheet({ mocks: baseMocks([CG_1]) });

    // SheetTitle renders a heading labelling the dialog.
    await waitFor(() => {
      expect(screen.getByText("Select cardgroup")).toBeInTheDocument();
    });
  });

  // S9: "Create new cardgroup…" link is present even when there are existing cardgroups.
  it("shows the inline create link when the connection is non-empty", async () => {
    renderSheet({ mocks: baseMocks([CG_1, CG_2]) });

    await screen.findByText("Spanish Vocab");

    expect(screen.getByRole("link", { name: /create new cardgroup/i })).toBeInTheDocument();
  });

  // S10: the inline create link's href encodes createReturnTo as the returnTo query parameter.
  it("builds the create link href with the encoded createReturnTo value", async () => {
    renderSheet({ mocks: baseMocks([CG_1]), createReturnTo: "/cards/new" });

    await screen.findByText("Spanish Vocab");

    const link = screen.getByRole("link", { name: /create new cardgroup/i });
    expect(link).toHaveAttribute(
      "href",
      `/cardgroups/new?returnTo=${encodeURIComponent("/cards/new")}`,
    );
    // Explicit suffix check per spec: createReturnTo="/cards/new" → %2Fcards%2Fnew
    expect(link.getAttribute("href")).toContain("%2Fcards%2Fnew");
  });

  // S14: Null myCardgroupsConnection in the server response emits a console.warn
  it("warns when myCardgroupsConnection arrives null from the server", async () => {
    const nullConnectionMock: MockedResponse = {
      request: { query: MyCardgroupsConnectionDocument, variables: { first: 100 } },
      result: {
        data: {
          myCardgroupsConnection: null,
        },
      },
    };

    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderSheet({ mocks: [nullConnectionMock] });

      await waitFor(() => {
        expect(consoleWarnSpy).toHaveBeenCalledWith(
          expect.stringContaining("[cardgroup-picker-sheet]"),
        );
      });
    } finally {
      consoleWarnSpy.mockRestore();
    }
  });

  // S11: clicking the inline create link calls onOpenChange(false).
  it("calls onOpenChange(false) when the inline create link is clicked", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderSheet({ mocks: baseMocks([CG_1]) });

    await screen.findByText("Spanish Vocab");

    await user.click(screen.getByRole("link", { name: /create new cardgroup/i }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
