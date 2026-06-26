// @vitest-environment happy-dom
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AdminMasterCardsConnectionDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import { renderWithIntl } from "@/test/render-with-intl";
import jaMessages from "../../../../../../../messages/ja.json";
import { MasterCardsClient } from "./master-cards-client";
import { masterCardsDefaultVars } from "./queries";

vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function Mock(
      { children }: { children: React.ReactNode },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return <div>{children}</div>;
    }),
  };
});

const MASTER_ID = "m-1";
const edge = (id: string, front: string) => ({
  __typename: "MasterCardEdge" as const,
  cursor: id,
  node: {
    __typename: "MasterCard" as const,
    id,
    masterCardgroupId: MASTER_ID,
    front,
    back: `${front}-back`,
    position: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
});

const seed = {
  request: {
    query: AdminMasterCardsConnectionDocument,
    variables: masterCardsDefaultVars(MASTER_ID),
  },
  result: {
    data: {
      adminMasterCardsConnection: {
        __typename: "MasterCardConnection" as const,
        edges: [edge("c-1", "apple")],
        pageInfo: {
          __typename: "PageInfo" as const,
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: "c-1",
          endCursor: "c-1",
        },
        totalCount: 1,
      },
    },
  },
};

// IntersectionObserver stub — capture the latest observer callback so a test
// can drive the sentinel into view and trigger the hook's fetchMore. The hook
// only constructs an observer when `hasNextPage` is true, so the two seeded
// tests below (hasNextPage: false) never touch it.
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

beforeEach(() => {
  ioCallbacks = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

type SectionHeader =
  | React.ReactNode
  | ((args: {
      totalCount: number;
      onAddCard: () => void;
      onBatchImport: () => void;
    }) => React.ReactNode);

function renderClient(sectionHeader?: SectionHeader) {
  renderWithIntl(
    <MockedProvider mocks={[seed]}>
      <UndoDeleteProvider>
        <MasterCardsClient
          masterId={MASTER_ID}
          deckName="Deck One"
          initialEdges={[edge("c-1", "apple")]}
          initialPageInfo={seed.result.data.adminMasterCardsConnection.pageInfo}
          initialTotalCount={1}
          sectionHeader={sectionHeader}
        />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
}

describe("<MasterCardsClient>", () => {
  it("renders the seeded card and passes the live total count to the section header", async () => {
    renderClient(({ totalCount }) => <span data-testid="hdr-count">{totalCount}</span>);
    expect(screen.getByText("apple")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId("hdr-count")).toHaveTextContent("1"));
  });

  it("opens the add-card sheet when a flamingo:add-master-card event targets this master", async () => {
    renderClient();
    act(() => {
      window.dispatchEvent(
        new CustomEvent("flamingo:add-master-card", { detail: { masterId: MASTER_ID } }),
      );
    });
    expect(await screen.findByLabelText(/front/i)).toBeInTheDocument();
  });

  it("ignores a flamingo:add-master-card event addressed to a different master", async () => {
    renderClient();
    act(() => {
      window.dispatchEvent(
        new CustomEvent("flamingo:add-master-card", { detail: { masterId: "other-master" } }),
      );
    });
    // The card list is present, but the add-card sheet's front field never opens.
    expect(screen.getByText("apple")).toBeInTheDocument();
    expect(screen.queryByLabelText(/front/i)).not.toBeInTheDocument();
  });

  it("opens the add-card sheet via the section-header onAddCard callback", async () => {
    renderClient(({ onAddCard }) => (
      <button type="button" data-testid="hdr-add" onClick={onAddCard}>
        add
      </button>
    ));
    await userEvent.click(screen.getByTestId("hdr-add"));
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
  });

  it("opens the batch-import sheet via the section-header onBatchImport callback", async () => {
    renderClient(({ onBatchImport }) => (
      <button type="button" data-testid="hdr-import" onClick={onBatchImport}>
        import
      </button>
    ));
    await userEvent.click(screen.getByTestId("hdr-import"));
    // The shared batch-import wizard renders its paste textarea inside the sheet.
    expect(await screen.findByTestId("batch-import-payload")).toBeInTheDocument();
  });

  it("renders the localized fetchMore-failure banner under the ja locale", async () => {
    const pageInfoWithNext = {
      __typename: "PageInfo" as const,
      hasNextPage: true,
      hasPreviousPage: false,
      startCursor: "c-1",
      endCursor: "c-1",
    };
    const seedWithNext = {
      request: {
        query: AdminMasterCardsConnectionDocument,
        variables: masterCardsDefaultVars(MASTER_ID),
      },
      result: {
        data: {
          adminMasterCardsConnection: {
            __typename: "MasterCardConnection" as const,
            edges: [edge("c-1", "apple")],
            pageInfo: pageInfoWithNext,
            totalCount: 1,
          },
        },
      },
    };
    // The next page fails with ONLY a field-level BAD_USER_INPUT error, the one
    // shape getBackendErrorBanner returns undefined for — so the hook falls back
    // to the caller-supplied localized message (the path issue #535 fixes).
    // A plain network error would instead hit getBackendErrorBanner's own
    // NETWORK_ERROR banner and never reach the fallback.
    const { CombinedGraphQLErrors } = await import("@apollo/client/errors");
    const fieldOnlyError = new CombinedGraphQLErrors({
      data: null,
      errors: [{ message: "bad input", extensions: { code: "BAD_USER_INPUT", field: "search" } }],
    });
    const fetchMoreErrorMock = {
      request: {
        query: AdminMasterCardsConnectionDocument,
        variables: { ...masterCardsDefaultVars(MASTER_ID), after: "c-1", search: null },
      },
      error: fieldOnlyError,
    };

    renderWithIntl(
      <MockedProvider mocks={[seedWithNext, fetchMoreErrorMock]}>
        <UndoDeleteProvider>
          <MasterCardsClient
            masterId={MASTER_ID}
            deckName="Deck One"
            initialEdges={[edge("c-1", "apple")]}
            initialPageInfo={pageInfoWithNext}
            initialTotalCount={1}
          />
        </UndoDeleteProvider>
      </MockedProvider>,
      { locale: "ja", messages: jaMessages },
    );

    // Drive the sentinel into view → the hook fires fetchMore, which fails.
    fireIntersect();

    const banner = await screen.findByTestId("cards-fetch-more-error");
    // The localized ja copy now flows through next-intl rather than a hardcoded
    // English literal. Reference the catalog string to keep CJK out of source
    // (language-policy), and assert the old English literal is absent.
    expect(banner).toHaveTextContent(jaMessages.Cards.fetchMoreFailed);
    expect(banner).not.toHaveTextContent("Could not load more cards");
  });
});

// ---------------------------------------------------------------------------
// Mobile search takeover — flamingo:open-search
// ---------------------------------------------------------------------------

describe("<MasterCardsClient> mobile search takeover", () => {
  it("opens the takeover on flamingo:open-search and renders the input", async () => {
    renderClient();
    await screen.findByText("apple");

    expect(screen.queryByTestId("search-takeover")).not.toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new CustomEvent("flamingo:open-search"));
    });

    expect(screen.getByTestId("search-takeover")).toBeInTheDocument();
    expect(screen.getByTestId("search-takeover-input")).toBeInTheDocument();
  });

  it("hides the desktop search input on mobile via hidden md:block", () => {
    renderClient();
    const wrapper = screen.getByTestId("cards-search-input").closest("div.mb-3");
    expect(wrapper).toHaveClass("hidden", "md:block");
  });
});
