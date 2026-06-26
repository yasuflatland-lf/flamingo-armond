// @vitest-environment jsdom
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  MergeMasterCardgroupMutation,
  MergeMasterCardgroupPreviewQuery,
} from "@/app/catalog/queries";
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

function previewMock(
  masterId: string,
  payload: { __typename: string; [key: string]: unknown },
): MockedResponse {
  return {
    request: {
      query: MergeMasterCardgroupPreviewQuery,
      variables: {
        input: { masterCardgroupId: masterId, cardgroupId: TARGET_CARDGROUP_ID },
      },
    },
    result: {
      data: {
        mergeMasterCardgroupPreview: payload,
      },
    },
  };
}

function renderSheet(opts: {
  mocks: MockedResponse[];
  onMerged?: (result: { addedCount: number; updatedCount: number }) => void;
  onOpenChange?: (open: boolean) => void;
}) {
  const onOpenChange = opts.onOpenChange ?? vi.fn();
  const onMerged = opts.onMerged ?? vi.fn();

  renderWithIntl(
    <MockedProvider mocks={opts.mocks}>
      <MergeFromCatalogSheet
        open
        onOpenChange={onOpenChange}
        targetCardgroupId={TARGET_CARDGROUP_ID}
        targetCardgroupName="My Deck"
        onMerged={onMerged}
      />
    </MockedProvider>,
  );

  return { onOpenChange, onMerged };
}

beforeEach(() => {
  vi.spyOn(ApolloClient.prototype, "refetchQueries").mockResolvedValue([]);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<MergeFromCatalogSheet>", () => {
  it("renders catalog rows in a FormSheet and filters via debounced search variables", async () => {
    const user = userEvent.setup();
    renderSheet({ mocks: [BASE_CATALOG, SEARCH_CATALOG] });

    expect(await screen.findByRole("dialog", { name: "Merge from catalog" })).toBeInTheDocument();
    expect(await screen.findByTestId("merge-from-catalog-row-master-1")).toBeInTheDocument();
    expect(screen.getByText("Business English")).toBeInTheDocument();

    await user.type(screen.getByTestId("merge-from-catalog-search"), "kanji");

    expect(await screen.findByTestId("merge-from-catalog-row-master-2")).toBeInTheDocument();
    expect(screen.getByText("JLPT Kanji")).toBeInTheDocument();
  });

  it("selecting a row previews the diff and confirming merges", async () => {
    const user = userEvent.setup();
    const onMerged = vi.fn();
    const { onOpenChange } = renderSheet({
      onMerged,
      mocks: [
        BASE_CATALOG,
        previewMock("master-1", {
          __typename: "MergeMasterCardgroupPreview",
          addedCount: 312,
          updatedCount: 508,
        }),
        mergeMock("success"),
      ],
    });

    await user.click(await screen.findByTestId("merge-from-catalog-row-master-1"));
    const confirmBtn = await screen.findByTestId("merge-review-confirm");
    expect(confirmBtn).toHaveTextContent("820");
    expect(screen.getByTestId("merge-review-added")).toHaveTextContent("312");

    await user.click(confirmBtn);
    await waitFor(() => expect(onMerged).toHaveBeenCalledWith({ addedCount: 3, updatedCount: 1 }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows an error banner in the review area when the preview returns not_found", async () => {
    const user = userEvent.setup();
    renderSheet({
      mocks: [
        BASE_CATALOG,
        previewMock("master-1", { __typename: "MasterNotFoundError", message: "gone" }),
      ],
    });

    await user.click(await screen.findByTestId("merge-from-catalog-row-master-1"));
    const banner = await screen.findByTestId("merge-from-catalog-error");
    expect(banner).toHaveTextContent(/no longer available/i);
    expect(screen.getByTestId("merge-from-catalog-review-back")).toBeInTheDocument();
  });

  it.each([
    ["not_found", /catalog deck is no longer available/i],
    ["unauthenticated", /session expired/i],
    ["forbidden", /do not have permission/i],
    ["rejected", /merge failed, please try again/i],
  ] as const)("keeps the sheet open and shows a localized banner for merge %s", async (outcome, copy) => {
    const user = userEvent.setup();
    const { onMerged, onOpenChange } = renderSheet({
      mocks: [
        BASE_CATALOG,
        previewMock("master-1", {
          __typename: "MergeMasterCardgroupPreview",
          addedCount: 3,
          updatedCount: 1,
        }),
        mergeMock(outcome),
      ],
    });

    await user.click(await screen.findByTestId("merge-from-catalog-row-master-1"));
    await user.click(await screen.findByTestId("merge-review-confirm"));

    const banner = await screen.findByTestId("merge-from-catalog-error");
    expect(banner).toHaveTextContent(copy);
    expect(screen.getByRole("dialog", { name: "Merge from catalog" })).toBeInTheDocument();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(onMerged).not.toHaveBeenCalled();
  });

  it("disables the confirm button while the merge mutation is in flight", async () => {
    const user = userEvent.setup();
    const pendingMergeMock: MockedResponse = {
      ...mergeMock("success"),
      delay: Infinity,
    };
    renderSheet({
      mocks: [
        BASE_CATALOG,
        previewMock("master-1", {
          __typename: "MergeMasterCardgroupPreview",
          addedCount: 3,
          updatedCount: 1,
        }),
        pendingMergeMock,
      ],
    });

    await user.click(await screen.findByTestId("merge-from-catalog-row-master-1"));
    void user.click(await screen.findByTestId("merge-review-confirm"));

    await waitFor(() => {
      expect(screen.getByTestId("merge-review-confirm")).toBeDisabled();
    });
  });
});
