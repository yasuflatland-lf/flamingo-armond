// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../__tests__/utils/mock-apollo-paginated";
import enMessages from "../../../../messages/en.json";
import { AdminMastersClient } from "./admin-masters-client";
import { ADMIN_MASTERS_PAGE_SIZE, AdminCreateMasterMutation, AdminMastersQuery } from "./queries";

const M = enMessages.AdminMasters;

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const push = vi.fn();
vi.mock("next/navigation", async (orig) => ({
  ...(await orig<typeof import("next/navigation")>()),
  useRouter: () => ({ push, refresh: vi.fn() }),
}));

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
  push.mockClear();
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

// Mirror the backend opaque cursor envelope (cursor.Encode in Go): "v1:" + base64(id).
// The list edge cursor is deliberately NOT the raw node id, so a client that resolves
// edges by `cursor` instead of `node.id` breaks. Encoding the mock cursors keeps the
// fixture faithful to the real backend and guards the "Master not found." regression.
function encodeCursor(id: string): string {
  return `v1:${btoa(id)}`;
}

function node(id: string, over: Record<string, unknown> = {}) {
  return {
    __typename: "MasterCardgroup" as const,
    id,
    name: `Deck ${id}`,
    description: null,
    version: 1,
    status: "DRAFT" as const,
    isDefaultStarter: false,
    sortOrder: 0,
    cardCount: 5,
    ...over,
  };
}

function connection(
  ids: string[],
  hasNextPage = false,
  overrides: Record<string, Record<string, unknown>> = {},
) {
  const first = ids[0];
  const last = ids[ids.length - 1];
  return {
    __typename: "MasterCatalogConnection" as const,
    edges: ids.map((id) => ({
      __typename: "MasterCatalogEdge" as const,
      cursor: encodeCursor(id),
      node: node(id, overrides[id] ?? {}),
    })),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: first ? encodeCursor(first) : null,
      endCursor: last ? encodeCursor(last) : null,
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

function listMock(ids: string[], overrides: Record<string, Record<string, unknown>> = {}) {
  return {
    request: { query: AdminMastersQuery, variables: BASE_VARS },
    result: { data: { adminMasters: connection(ids, false, overrides) } },
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

  it("navigates to the edit route after a successful create", async () => {
    sheetState = { mode: "new" };
    const user = userEvent.setup();
    const createInput = {
      name: "Brand New",
      description: null,
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
              id: "m-created-1",
              name: "Brand New",
              description: null,
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
    const nameField = await screen.findByTestId("master-field-name");
    await user.type(nameField, "Brand New");
    await user.click(screen.getByTestId("master-form-submit"));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/admin/masters/m-created-1/edit"));
    expect(toast.success).toHaveBeenCalledWith(M.createSuccess);
  });

  it("does not carry optimisticResponse in the publish or unpublish mutation calls", () => {
    // AdminPublishMaster returns a typed-error union (PublishMasterCardgroupSuccess |
    // MasterCardgroupEmptyError); Apollo v4 does not consistently roll back optimistic
    // writes on typed GraphQL errors, so neither toggle mutation may carry one.
    // See .claude/rules/pagination.md "Drop optimisticResponse ...".
    // After the useMasterMutations refactor the runPublish/runUnpublish calls live in
    // the hook file, not in the client — check there so the guard remains meaningful.
    const source = readFileSync(
      join(process.cwd(), "src/app/admin/masters/use-master-mutations.ts"),
      "utf8",
    );
    for (const call of ["runPublish({", "runUnpublish({"]) {
      const start = source.indexOf(call);
      expect(start).toBeGreaterThanOrEqual(0);
      const end = source.indexOf("});", start);
      expect(end).toBeGreaterThan(start);
      expect(source.slice(start, end)).not.toContain("optimisticResponse");
    }
  });

  // -------------------------------------------------------------------------
  // Mobile search takeover — flamingo:open-search opens the bar; the desktop
  // input is gated mobile-off so the two do not double up on mobile.
  // -------------------------------------------------------------------------

  it("opens the takeover on flamingo:open-search and renders the input", async () => {
    renderWithIntl(
      <MockedProvider mocks={[listMock(["m-1"])]}>
        <AdminMastersClient />
      </MockedProvider>,
    );
    await screen.findByTestId("admin-masters-list");

    expect(screen.queryByTestId("search-takeover")).not.toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new CustomEvent("flamingo:open-search"));
    });

    expect(screen.getByTestId("search-takeover")).toBeInTheDocument();
    expect(screen.getByTestId("search-takeover-input")).toBeInTheDocument();
  });

  it("gates the desktop search input mobile-off (hidden md:block)", async () => {
    renderWithIntl(
      <MockedProvider mocks={[listMock(["m-1"])]}>
        <AdminMastersClient />
      </MockedProvider>,
    );
    await screen.findByTestId("admin-masters-list");

    const desktop = screen.getByRole("searchbox", { name: /search masters/i });
    expect(desktop.parentElement).toHaveClass("hidden", "md:block");
  });
});
