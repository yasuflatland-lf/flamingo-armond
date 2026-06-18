// @vitest-environment jsdom
/**
 * Broad page-level test for the MasterManagementClient tree.
 * Exercises the full card-editor integration: SSR-seeded render and
 * live-count propagation from the cards client up to the header badge
 * (onTotalCountChange → liveCount → MasterEditHeader).
 *
 * Mirror: frontend/__tests__/admin-masters-edit.test.tsx (provider stack)
 * Mirror: frontend/__tests__/cards-bulk-delete.test.tsx (interaction style)
 */

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
  }),
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
  // UndoDeleteProvider calls usePathname to detect navigation flushes.
  usePathname: () => "/admin/masters/m-broad-1/edit",
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
// CardRow wraps each card in SwipeableRow; stub it to avoid gesture-lib load.
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

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { masterCardsDefaultVars } from "@/app/admin/masters/[id]/edit/cards/queries";
import { MasterManagementClient } from "@/app/admin/masters/[id]/edit/master-management-client";
import {
  AdminCreateMasterCardDocument,
  AdminMasterCardsConnectionDocument,
} from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import enMessages from "../messages/en.json";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const ID = "m-broad-1";
const C1 = "c-broad-1";

const DECK = {
  __typename: "MasterCardgroup" as const,
  id: ID,
  name: "Broad Test Deck",
  description: null,
  language: null,
  level: null,
  category: null,
  coverImageUrl: null,
  source: null,
  version: 1,
  status: "DRAFT" as const,
  isDefaultStarter: false,
  sortOrder: 0,
  cardCount: 1,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

const INITIAL_EDGE = {
  __typename: "MasterCardEdge" as const,
  cursor: C1,
  node: {
    __typename: "MasterCard" as const,
    id: C1,
    masterCardgroupId: ID,
    front: "apple",
    back: "apple-back",
    position: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
};

const INITIAL_PAGE_INFO = {
  __typename: "PageInfo" as const,
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: C1,
  endCursor: C1,
};

// MockedProvider mock: the connection query fired by useMasterCardsConnection
// on mount (seeded from the SSR props, no network round-trip needed here — the
// mock is provided so any background refetch does not generate an unmatched warn).
const connMock = {
  request: {
    query: AdminMasterCardsConnectionDocument,
    variables: masterCardsDefaultVars(ID),
  },
  result: {
    data: {
      adminMasterCardsConnection: {
        __typename: "MasterCardConnection",
        edges: [INITIAL_EDGE],
        pageInfo: INITIAL_PAGE_INFO,
        totalCount: 1,
      },
    },
  },
};

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

// Stub IntersectionObserver so the IO-driven pagination sentinel is a no-op.
class FakeIO {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

beforeEach(() => {
  vi.stubGlobal("IntersectionObserver", FakeIO);
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

function renderClient(mocks: object[] = [connMock]) {
  render(
    <MockedProvider mocks={mocks as never}>
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        <UndoDeleteProvider>
          <MasterManagementClient
            master={DECK}
            initialEdges={[INITIAL_EDGE]}
            initialPageInfo={INITIAL_PAGE_INFO}
            initialTotalCount={1}
          />
        </UndoDeleteProvider>
      </NextIntlClientProvider>
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("MasterManagementClient — broad page integration", () => {
  // T1: Seed render — the seeded card's front text is visible and the header
  // reflects the initial total count (1 card).
  it("renders the seeded card and shows the initial card count in the header", async () => {
    renderClient();

    // The seeded card's front text appears in the card list.
    expect(screen.getByText("apple")).toBeInTheDocument();

    // The header status badge region renders the AdminMasters.cardCount message.
    // With count=1 the ICU plural resolves to "1 card".
    await waitFor(() => {
      expect(screen.getByText("1 card")).toBeInTheDocument();
    });
  });

  // T2: Live count on create — open the add-card sheet, fill front + back,
  // submit with a mocked AdminCreateMasterCard success. The create's cache write
  // bumps the connection's totalCount, which flows up via onTotalCountChange,
  // and the header updates from "1 card" to "2 cards".
  it("updates the header count from 1 to 2 after a successful card create", async () => {
    const user = userEvent.setup();
    const C2 = "c-broad-2";

    const createMock = {
      request: {
        query: AdminCreateMasterCardDocument,
        variables: {
          input: {
            masterCardgroupId: ID,
            front: "banana",
            back: "banana-back",
          },
        },
      },
      result: {
        data: {
          adminCreateMasterCard: {
            __typename: "CreateMasterCardSuccess",
            masterCard: {
              __typename: "MasterCard",
              id: C2,
              masterCardgroupId: ID,
              front: "banana",
              back: "banana-back",
              position: 1,
              createdAt: "2026-01-02T00:00:00Z",
              updatedAt: "2026-01-02T00:00:00Z",
            },
          },
        },
      },
    };

    renderClient([connMock, createMock]);

    // Confirm initial count before mutation.
    await waitFor(() => {
      expect(screen.getByText("1 card")).toBeInTheDocument();
    });

    // Open the add-card sheet via the toolbar "Add card" button.
    await user.click(screen.getByRole("button", { name: /add card/i }));

    // The CardForm renders labeled inputs: "Front" and "Back".
    const frontInput = await screen.findByLabelText(/front/i);
    const backInput = screen.getByLabelText(/back/i);

    await user.type(frontInput, "banana");
    await user.type(backInput, "banana-back");

    // Submit the form. The label for the create button resolves to "Add" (the
    // default for mode="create" when no submitLabel prop is passed).
    await user.click(screen.getByRole("button", { name: /^add$/i }));

    // After a successful create the cache is updated (+1 to totalCount) and the
    // useEffect in MasterCardsClient calls onTotalCountChange(2), which updates
    // liveCount in MasterManagementClient and re-renders MasterEditHeader with
    // count=2. The ICU plural for count=2 resolves to "2 cards".
    await waitFor(() => {
      expect(screen.getByText("2 cards")).toBeInTheDocument();
    });
  });
});
