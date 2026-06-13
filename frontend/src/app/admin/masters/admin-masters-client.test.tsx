// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import { AdminMastersClient } from "./admin-masters-client";
import { ADMIN_MASTERS_PAGE_SIZE, AdminMastersQuery } from "./queries";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

vi.mock("@/lib/url/use-sheet-search-param", () => ({
  useSheetSearchParam: () => ({ state: { mode: "closed" }, open: vi.fn(), close: vi.fn() }),
}));

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
});
