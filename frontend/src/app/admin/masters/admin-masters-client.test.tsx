// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { AdminMastersClient } from "./admin-masters-client";
import {
  ADMIN_MASTERS_PAGE_SIZE,
  AdminCreateMasterMutation,
  AdminDeleteMasterMutation,
  AdminMastersQuery,
} from "./queries";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

let sheetState: { mode: "closed" } | { mode: "new" } | { mode: "edit"; id: string } = {
  mode: "closed",
};
const sheetClose = vi.fn();
const sheetOpen = vi.fn();
vi.mock("@/lib/url/use-sheet-search-param", () => ({
  useSheetSearchParam: () => ({ state: sheetState, open: sheetOpen, close: sheetClose }),
}));
beforeEach(() => {
  sheetState = { mode: "closed" };
  sheetOpen.mockClear();
  sheetClose.mockClear();
});

class FakeIO {
  root = null;
  rootMargin = "";
  thresholds = [];
  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() {
    return [];
  }
}

beforeEach(() => {
  vi.stubGlobal("IntersectionObserver", FakeIO as unknown as typeof IntersectionObserver);
});

function node(id: string, over: Record<string, unknown> = {}) {
  return {
    __typename: "MasterCardgroup" as const,
    id,
    name: `Deck ${id}`,
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
    cardCount: 5,
    ...over,
  };
}

function connection(ids: string[], hasNextPage = false) {
  return {
    __typename: "MasterCatalogConnection" as const,
    edges: ids.map((id) => ({
      __typename: "MasterCatalogEdge" as const,
      cursor: id,
      node: node(id),
    })),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: ids[0] ?? null,
      endCursor: ids[ids.length - 1] ?? null,
    },
    totalCount: ids.length,
  };
}

const BASE_VARS = {
  first: ADMIN_MASTERS_PAGE_SIZE,
  search: null,
  orderBy: "SORT_ORDER",
  orderDirection: "ASC",
};

function listMock(ids: string[]) {
  return {
    request: { query: AdminMastersQuery, variables: BASE_VARS },
    result: { data: { adminMasters: connection(ids) } },
  };
}

let leak: ApolloMockLeakSpyResult;
beforeEach(() => {
  leak = installApolloMockLeakSpy({ operationNames: ["AdminMasters"] });
});
afterEach(() => {
  leak.assertNoLeaks();
  leak.teardown();
});

describe("AdminMastersClient", () => {
  it("renders the list after the initial query resolves", async () => {
    renderWithIntl(
      <MockedProvider mocks={[listMock(["m-1", "m-2"])]}>
        <AdminMastersClient />
      </MockedProvider>,
    );
    expect(await screen.findByTestId("admin-masters-list")).toBeInTheDocument();
    expect(screen.getByText("Deck m-1")).toBeInTheDocument();
  });

  it("shows the empty state when there are no masters", async () => {
    renderWithIntl(
      <MockedProvider mocks={[listMock([])]}>
        <AdminMastersClient />
      </MockedProvider>,
    );
    expect(await screen.findByTestId("admin-masters-empty")).toBeInTheDocument();
  });

  it("renders the FORBIDDEN banner without a Retry", async () => {
    const { CombinedGraphQLErrors } = await import("@apollo/client/errors");
    const forbidden = new CombinedGraphQLErrors({
      data: null,
      errors: [{ message: "forbidden", extensions: { code: "FORBIDDEN" } }],
    });
    renderWithIntl(
      <MockedProvider
        mocks={[{ request: { query: AdminMastersQuery, variables: BASE_VARS }, error: forbidden }]}
      >
        <AdminMastersClient />
      </MockedProvider>,
    );
    expect(await screen.findByTestId("admin-masters-query-error")).toBeInTheDocument();
  });

  it("prepends the newly created deck to the list", async () => {
    sheetState = { mode: "new" };
    const user = userEvent.setup();
    const createInput = {
      name: "Brand New",
      description: null,
      language: null,
      level: null,
      category: null,
      coverImageUrl: null,
      source: null,
      isDefaultStarter: false,
      sortOrder: null,
    };
    const createMock = {
      request: { query: AdminCreateMasterMutation, variables: { input: createInput } },
      result: {
        data: {
          adminCreateMasterCardgroup: {
            __typename: "CreateMasterCardgroupSuccess" as const,
            master: {
              __typename: "MasterCardgroup" as const,
              id: "new-1",
              name: "Brand New",
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
              cardCount: 0,
            },
          },
        },
      },
    };
    renderWithIntl(
      <MockedProvider mocks={[listMock(["m-1"]), createMock]}>
        <AdminMastersClient />
      </MockedProvider>,
    );
    // The create drawer renders the form.
    const nameField = await screen.findByTestId("master-field-name");
    await user.type(nameField, "Brand New");
    await user.click(screen.getByTestId("master-form-submit"));
    // The writeQuery prepend updates the search:null connection the list reads.
    expect(await screen.findByText("Brand New")).toBeInTheDocument();
    expect(sheetClose).toHaveBeenCalled();
  });

  it("removes the deleted deck from the list", async () => {
    sheetState = { mode: "edit", id: "m-1" };
    const user = userEvent.setup();
    const deleteMock = {
      request: { query: AdminDeleteMasterMutation, variables: { id: "m-1" } },
      result: { data: { adminDeleteMasterCardgroup: true } },
    };
    renderWithIntl(
      <MockedProvider mocks={[listMock(["m-1"]), deleteMock]}>
        <AdminMastersClient />
      </MockedProvider>,
    );
    // Wait for the loaded deck so the edit drawer's danger zone renders.
    expect(await screen.findByText("Deck m-1")).toBeInTheDocument();
    await user.click(screen.getByTestId("master-row-delete-trigger"));
    await user.click(await screen.findByTestId("master-delete-dialog-confirm"));
    await waitFor(() => expect(screen.queryByText("Deck m-1")).not.toBeInTheDocument());
    expect(toast.success).toHaveBeenCalled();
  });
});
