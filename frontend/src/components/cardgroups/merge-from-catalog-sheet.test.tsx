// @vitest-environment jsdom
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MergeMasterCardgroupMutation } from "@/app/catalog/queries";
import { MasterCatalogDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { MergeFromCatalogSheet } from "./merge-from-catalog-sheet";

vi.mock("@/hooks/use-mobile", () => ({
  useIsMobile: () => false,
}));

const TARGET_CARDGROUP_ID = "cg-target";

const BASE_CATALOG = {
  request: {
    query: MasterCatalogDocument,
    variables: { first: 20, search: null },
  },
  result: {
    data: {
      masterCatalog: {
        __typename: "MasterCatalogConnection",
        totalCount: 1,
        edges: [
          {
            __typename: "MasterCatalogEdge",
            cursor: "cursor-master-1",
            node: {
              __typename: "MasterCardgroup",
              id: "master-1",
              name: "Business English",
              description: "Professional vocabulary",
              cardCount: 42,
            },
          },
        ],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: "cursor-master-1",
          endCursor: "cursor-master-1",
        },
      },
    },
  },
} satisfies MockedResponse;

const SEARCH_CATALOG = {
  request: {
    query: MasterCatalogDocument,
    variables: { first: 20, search: "kanji" },
  },
  result: {
    data: {
      masterCatalog: {
        __typename: "MasterCatalogConnection",
        totalCount: 1,
        edges: [
          {
            __typename: "MasterCatalogEdge",
            cursor: "cursor-master-2",
            node: {
              __typename: "MasterCardgroup",
              id: "master-2",
              name: "JLPT Kanji",
              description: null,
              cardCount: 100,
            },
          },
        ],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: "cursor-master-2",
          endCursor: "cursor-master-2",
        },
      },
    },
  },
} satisfies MockedResponse;

function mergeMock(
  outcome: "success" | "not_found" | "unauthenticated" | "forbidden" | "rejected" = "success",
): MockedResponse {
  const request = {
    query: MergeMasterCardgroupMutation,
    variables: {
      input: {
        masterCardgroupId: "master-1",
        cardgroupId: TARGET_CARDGROUP_ID,
      },
    },
  };

  if (outcome === "success") {
    return {
      request,
      result: {
        data: {
          mergeMasterCardgroup: {
            __typename: "MergeMasterCardgroupSuccess",
            cardgroup: {
              __typename: "Cardgroup",
              id: TARGET_CARDGROUP_ID,
              name: "Target deck",
              updatedAt: "2026-06-26T00:00:00Z",
            },
            addedCount: 3,
            updatedCount: 1,
          },
        },
      },
    };
  }

  if (outcome === "not_found") {
    return {
      request,
      result: {
        data: {
          mergeMasterCardgroup: {
            __typename: "MasterNotFoundError",
            message: "gone",
          },
        },
      },
    };
  }

  if (outcome === "rejected") {
    return { request, error: new Error("network down") };
  }

  return {
    request,
    result: {
      errors: [
        {
          message: "denied",
          extensions: {
            code: outcome === "unauthenticated" ? "UNAUTHENTICATED" : "FORBIDDEN",
          },
        },
      ],
    },
  };
}

function renderSheet(mocks: MockedResponse[], props?: { onOpenChange?: (open: boolean) => void }) {
  const onOpenChange = props?.onOpenChange ?? vi.fn();
  const onMerged = vi.fn();

  renderWithIntl(
    <MockedProvider mocks={mocks}>
      <MergeFromCatalogSheet
        open
        onOpenChange={onOpenChange}
        targetCardgroupId={TARGET_CARDGROUP_ID}
        onMerged={onMerged}
      />
    </MockedProvider>,
  );

  return { onOpenChange, onMerged };
}

async function openConfirmDialog(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByTestId("merge-from-catalog-master-1"));
  return screen.findByRole("alertdialog");
}

beforeEach(() => {
  vi.spyOn(ApolloClient.prototype, "refetchQueries").mockResolvedValue([]);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<MergeFromCatalogSheet>", () => {
  it("renders catalog tiles in a FormSheet and filters via debounced search variables", async () => {
    const user = userEvent.setup();
    renderSheet([BASE_CATALOG, SEARCH_CATALOG]);

    expect(await screen.findByRole("dialog", { name: "Merge from catalog" })).toBeInTheDocument();
    expect(await screen.findByText("Business English")).toBeInTheDocument();

    await user.type(screen.getByTestId("merge-from-catalog-search"), "kanji");

    expect(await screen.findByText("JLPT Kanji")).toBeInTheDocument();
  });

  it("opens a warning dialog with destructive confirm copy when a catalog tile is selected", async () => {
    const user = userEvent.setup();
    renderSheet([BASE_CATALOG]);

    const dialog = await openConfirmDialog(user);

    expect(dialog).toHaveTextContent("Cards with the same term will be overwritten");
    expect(dialog).toHaveTextContent("learning progress is kept");

    const confirm = screen.getByRole("button", { name: "Merge" });
    expect(confirm).toHaveClass("bg-destructive", "text-destructive-foreground");
  });

  it("calls onMerged with counts and closes the dialog and sheet after a successful merge", async () => {
    const user = userEvent.setup();
    const { onMerged, onOpenChange } = renderSheet([BASE_CATALOG, mergeMock("success")]);

    await openConfirmDialog(user);
    await user.click(screen.getByRole("button", { name: "Merge" }));

    await waitFor(() => {
      expect(onMerged).toHaveBeenCalledWith({ added: 3, updated: 1 });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it.each([
    ["not_found", /catalog deck is no longer available/i],
    ["unauthenticated", /session expired/i],
    ["forbidden", /do not have permission/i],
    ["rejected", /merge failed, please try again/i],
  ] as const)("keeps the sheet open and shows a localized banner for %s", async (outcome, copy) => {
    const user = userEvent.setup();
    const { onMerged, onOpenChange } = renderSheet([BASE_CATALOG, mergeMock(outcome)]);

    await openConfirmDialog(user);
    await user.click(screen.getByRole("button", { name: "Merge" }));

    const banner = await screen.findByTestId("merge-from-catalog-error");
    expect(banner).toHaveTextContent(copy);
    expect(screen.getByRole("dialog", { name: "Merge from catalog" })).toBeInTheDocument();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(onMerged).not.toHaveBeenCalled();
  });
});
