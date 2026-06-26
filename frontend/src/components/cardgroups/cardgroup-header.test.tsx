// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  DeleteCardgroupDocument,
  MasterCatalogDocument,
  UpdateCardgroupDocument,
} from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupHeader } from "./cardgroup-header";

const mockPush = vi.fn();
const mockRefresh = vi.fn();
const onBatchImport = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

const CARDGROUP = { id: "cg-1", name: "Spanish Vocab" };

// CardgroupHeader always mounts <MergeFromCatalogSheet>, but the sheet passes
// `skip: !open` to useConnectionPagination, so the MasterCatalog query does NOT
// fire while the sheet is closed. This mock is therefore consumed only by the
// integration test that opens the sheet (which flips `open` true and lifts the
// skip); render-only tests that never open it pay no query cost.
// Variables mirror CATALOG_DEFAULT_VARS ({ first: 20, search: null }) and the
// node shape mirrors merge-from-catalog-sheet.test.tsx's BASE_CATALOG.
const BASE_CATALOG: MockedResponse = {
  request: {
    query: MasterCatalogDocument,
    variables: { first: 20, search: null },
  },
  maxUsageCount: Number.POSITIVE_INFINITY,
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
};

function makeDeleteMock(
  variables: { id: string },
  result: MockedResponse["result"],
): MockedResponse {
  return { request: { query: DeleteCardgroupDocument, variables }, result };
}

function makeUpdateMock(
  variables: { id: string; input: { name: string } },
  result: MockedResponse["result"],
  delay?: MockedResponse["delay"],
): MockedResponse {
  return { request: { query: UpdateCardgroupDocument, variables }, result, delay };
}

function renderHeader(mocks: MockedResponse[] = [], totalCount = 5) {
  renderWithIntl(
    <MockedProvider mocks={[BASE_CATALOG, ...mocks]}>
      <CardgroupHeader
        cardgroup={CARDGROUP}
        totalCount={totalCount}
        onBatchImport={onBatchImport}
      />
    </MockedProvider>,
  );
}

async function openDeleteDialog(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
  await waitFor(() =>
    expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument(),
  );
  await user.click(screen.getByRole("menuitem", { name: /delete cardgroup/i }));
  await waitFor(() => expect(screen.getByRole("alertdialog")).toBeInTheDocument());
}

async function clickDeleteConfirm(user: ReturnType<typeof userEvent.setup>) {
  const confirmBtn = screen
    .getAllByRole("button", { name: /^delete$/i })
    .find((el) => el.closest("[role='alertdialog']"));
  if (!confirmBtn) throw new Error("Delete confirm button not found in dialog");
  await user.click(confirmBtn);
}

describe("<CardgroupHeader>", () => {
  beforeEach(() => {
    mockPush.mockClear();
    mockRefresh.mockClear();
    onBatchImport.mockClear();
  });

  it("renders the cardgroup name as h1", () => {
    renderHeader();
    expect(screen.getByRole("heading", { level: 1, name: /spanish vocab/i })).toBeInTheDocument();
  });

  it("renders the totalCount as muted metadata text in the app-bar row", () => {
    renderHeader([], 42);
    expect(screen.getByText("42 cards")).toBeInTheDocument();
  });

  it("renders the kebab trigger button", () => {
    renderHeader();
    expect(screen.getByRole("button", { name: /cardgroup options/i })).toBeInTheDocument();
  });

  it("does not render an inline title-row rename button (rename moved into the overflow menu)", () => {
    renderHeader();
    expect(screen.queryByRole("button", { name: /^rename$/i })).toBeNull();
  });

  it("kebab menu hosts Rename as the first item, plus Delete", async () => {
    const user = userEvent.setup();
    renderHeader();

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));

    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /^rename$/i })).toBeInTheDocument();
    });
    expect(screen.getAllByRole("menuitem")[0]).toHaveTextContent(/rename/i);
    expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument();
  });

  it("selecting Rename from the overflow menu opens the Rename cardgroup FormSheet", async () => {
    const user = userEvent.setup();
    renderHeader();

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await user.click(await screen.findByRole("menuitem", { name: /^rename$/i }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /rename cardgroup/i })).toBeInTheDocument();
    });
  });

  it("does not dismiss the rename FormSheet while save is submitting", async () => {
    const user = userEvent.setup();
    renderHeader([
      makeUpdateMock(
        { id: "cg-1", input: { name: "Spanish Vocab" } },
        {
          data: {
            updateCardgroup: {
              __typename: "UpdateCardgroupSuccess" as const,
              cardgroup: {
                __typename: "Cardgroup" as const,
                id: "cg-1",
                name: "Spanish Vocab",
                updatedAt: "2024-06-15T10:00:00.000Z",
              },
            },
          },
        },
        Infinity,
      ),
    ]);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await user.click(await screen.findByRole("menuitem", { name: /^rename$/i }));
    await user.click(await screen.findByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /saving/i })).toBeInTheDocument();
    });
    await user.keyboard("{Escape}");

    expect(screen.getByRole("heading", { name: /rename cardgroup/i })).toBeInTheDocument();
  });

  it("clicking Delete cardgroup opens the AlertDialog", async () => {
    const user = userEvent.setup();
    renderHeader();

    await openDeleteDialog(user);

    expect(
      screen.getByRole("heading", { level: 2, name: /^delete cardgroup$/i }),
    ).toBeInTheDocument();
  });

  it("cancel closes the delete dialog without firing mutation", async () => {
    const user = userEvent.setup();
    const deleteCalled = vi.fn();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        result: () => {
          deleteCalled();
          return { data: { deleteCardgroup: true } };
        },
      },
    ];
    renderHeader(mocks);

    await openDeleteDialog(user);

    await user.click(screen.getByRole("button", { name: /^cancel$/i }));

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(deleteCalled).not.toHaveBeenCalled();
  });

  it("confirm fires DeleteCardgroupMutation and navigates to /cardgroups", async () => {
    const user = userEvent.setup();
    const deleteCalled = vi.fn();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        result: () => {
          deleteCalled();
          return { data: { deleteCardgroup: true } };
        },
      },
    ];
    renderHeader(mocks);

    await openDeleteDialog(user);
    await clickDeleteConfirm(user);

    await waitFor(() => {
      expect(deleteCalled).toHaveBeenCalledOnce();
    });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups");
    });
    expect(mockRefresh).toHaveBeenCalledTimes(1);
  });

  it("delete UNAUTHENTICATED shows banner error and dialog stays open", async () => {
    const user = userEvent.setup();
    const mocks = [
      makeDeleteMock(
        { id: "cg-1" },
        {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      ),
    ];
    renderHeader(mocks);

    await openDeleteDialog(user);
    await clickDeleteConfirm(user);

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("renders an inline back link to /cardgroups and count meta, no status", () => {
    renderHeader([], 12);
    expect(screen.getByRole("link", { name: /back to cardgroups/i })).toHaveAttribute(
      "href",
      "/cardgroups",
    );
    expect(screen.getByText("12 cards")).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("overflow menu holds rename, batch import, and delete", async () => {
    const user = userEvent.setup();
    renderHeader([], 12);
    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    expect(screen.getByRole("menuitem", { name: /^rename$/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /batch import/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /delete cardgroup/i })).toBeInTheDocument();
  });

  it("overflow menu 'Batch import' item calls onBatchImport", async () => {
    const user = userEvent.setup();
    renderHeader([], 12);
    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));
    await user.click(screen.getByRole("menuitem", { name: /batch import/i }));
    expect(onBatchImport).toHaveBeenCalledTimes(1);
  });

  it("overflow menu hosts a 'Merge from catalog' item that opens the merge sheet", async () => {
    const user = userEvent.setup();
    renderHeader([], 12);

    await user.click(screen.getByRole("button", { name: /cardgroup options/i }));

    const mergeItem = await screen.findByTestId("cardgroup-merge-menuitem");
    expect(mergeItem).toBeInTheDocument();
    expect(mergeItem).toHaveTextContent(/merge from catalog/i);

    await user.click(mergeItem);

    // The merge sheet opens — assert on a stable element it renders (the catalog
    // search input), proving the wiring from menu item → MergeFromCatalogSheet.
    expect(await screen.findByTestId("merge-from-catalog-search")).toBeInTheDocument();
  });

  it("delete network rejection shows error banner and dialog stays open", async () => {
    const user = userEvent.setup();
    const mocks: MockedResponse[] = [
      {
        request: { query: DeleteCardgroupDocument, variables: { id: "cg-1" } },
        error: new Error("Network error: failed to fetch"),
      },
    ];
    renderHeader(mocks);

    await openDeleteDialog(user);
    await clickDeleteConfirm(user);

    await waitFor(() => {
      expect(screen.getByText("Could not reach the server. Please try again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });
});
