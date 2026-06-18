// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
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

function render(onTotalCountChange = vi.fn()) {
  renderWithIntl(
    <MockedProvider mocks={[seed]}>
      <UndoDeleteProvider>
        <MasterCardsClient
          masterId={MASTER_ID}
          deckName="Deck One"
          initialEdges={[edge("c-1", "apple")]}
          initialPageInfo={seed.result.data.adminMasterCardsConnection.pageInfo}
          initialTotalCount={1}
          onTotalCountChange={onTotalCountChange}
        />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
  return onTotalCountChange;
}

describe("<MasterCardsClient>", () => {
  it("renders the seeded card and reports the initial total count upward", async () => {
    const onTotalCountChange = render();
    expect(screen.getByText("apple")).toBeInTheDocument();
    await waitFor(() => expect(onTotalCountChange).toHaveBeenCalledWith(1));
  });

  it("opens the add-card sheet from the toolbar", async () => {
    render();
    await userEvent.click(screen.getByRole("button", { name: /add card/i }));
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
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
            onTotalCountChange={vi.fn()}
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
